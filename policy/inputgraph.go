package policy

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/containerd/platforms"
	gwpb "github.com/moby/buildkit/frontend/gateway/pb"
	"github.com/moby/buildkit/solver/pb"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	maxMaterialDepth = 24
)

type InputGraph struct {
	verifier        PolicyVerifierProvider
	defaultPlatform *ocispecs.Platform
	logf            func(logrus.Level, string)
	root            *graphNode
}

type graphNode struct {
	path     string
	source   *pb.SourceOp
	platform *ocispecs.Platform
	digest   map[string]string

	resp     *gwpb.ResolveSourceMetaResponse
	input    Input
	unknowns []string
	children []*graphNode
}

func NewInputGraph(ctx context.Context, verifier PolicyVerifierProvider, src *gwpb.ResolveSourceMetaResponse, platform *ocispecs.Platform, logf func(logrus.Level, string)) (*InputGraph, error) {
	if src == nil || src.Source == nil {
		return nil, errors.New("source metadata response is required")
	}
	g := &InputGraph{
		verifier:        verifier,
		defaultPlatform: clonePlatform(platform),
		logf:            logf,
		root: &graphNode{
			path:     "",
			source:   cloneSourceOp(src.Source),
			platform: clonePlatform(platform),
			resp:     cloneSourceMetaResponse(src),
		},
	}
	g.root.resp.Source = cloneSourceOp(src.Source)
	if err := g.Rebuild(ctx); err != nil {
		return nil, err
	}
	return g, nil
}

func (g *InputGraph) Input() Input {
	if g == nil || g.root == nil {
		return Input{}
	}
	return g.root.input
}

func (g *InputGraph) Unknowns() []string {
	if g == nil || g.root == nil {
		return nil
	}
	var out []string
	g.collectUnknowns(g.root, "", &out)
	return uniqueStrings(out)
}

func (g *InputGraph) collectUnknowns(n *graphNode, prefix string, out *[]string) {
	if n == nil {
		return
	}
	for _, u := range n.unknowns {
		if prefix == "" {
			*out = append(*out, "input."+u)
		} else {
			*out = append(*out, "input."+prefix+"."+u)
		}
	}
	for i, ch := range n.children {
		next := materialPrefix(prefix, i)
		g.collectUnknowns(ch, next, out)
	}
}

func materialPrefix(prefix string, idx int) string {
	seg := fmt.Sprintf("image.provenance.materials[%d]", idx)
	if prefix == "" {
		return seg
	}
	return prefix + "." + seg
}

func (g *InputGraph) ApplyResponse(path string, resp *gwpb.ResolveSourceMetaResponse) error {
	if g == nil || g.root == nil {
		return errors.New("input graph is empty")
	}
	n := g.nodeByPath(path)
	if n == nil {
		return errors.Errorf("material path %q not found", path)
	}
	if resp == nil || resp.Source == nil {
		return errors.New("resolved source metadata response is empty")
	}
	n.source = cloneSourceOp(resp.Source)
	n.resp = cloneSourceMetaResponse(resp)
	n.resp.Source = cloneSourceOp(resp.Source)
	return nil
}

func (g *InputGraph) nodeByPath(path string) *graphNode {
	if path == "" {
		return g.root
	}
	n := g.root
	rem := path
	for rem != "" {
		if !strings.HasPrefix(rem, "materials[") {
			return nil
		}
		i, rest, ok := parseMaterialIndex(rem)
		if !ok || i < 0 || i >= len(n.children) {
			return nil
		}
		n = n.children[i]
		rem = strings.TrimPrefix(rest, ".")
	}
	return n
}

func parseMaterialIndex(s string) (int, string, bool) {
	if !strings.HasPrefix(s, "materials[") {
		return 0, "", false
	}
	base, rest, ok := strings.Cut(s, "]")
	if !ok {
		return 0, "", false
	}
	idx, err := strconv.Atoi(strings.TrimPrefix(base, "materials["))
	if err != nil {
		return 0, "", false
	}
	return idx, rest, true
}

func (g *InputGraph) Rebuild(ctx context.Context) error {
	stack := map[string]struct{}{}
	return g.buildNode(ctx, g.root, 0, stack)
}

func (g *InputGraph) buildNode(ctx context.Context, n *graphNode, depth int, stack map[string]struct{}) error {
	if n == nil {
		return nil
	}
	if depth > maxMaterialDepth {
		return errors.Errorf("provenance materials depth exceeds limit %d", maxMaterialDepth)
	}
	if n.source == nil {
		return errors.New("node source is empty")
	}
	if n.resp == nil {
		n.resp = &gwpb.ResolveSourceMetaResponse{
			Source: cloneSourceOp(n.source),
		}
	}
	if n.resp.Source == nil {
		n.resp.Source = cloneSourceOp(n.source)
	}
	pl := n.platform
	if pl == nil {
		pl = g.defaultPlatform
	}

	inp, unk, err := SourceToInputWithLogger(ctx, g.verifier, n.resp, pl, g.logf)
	if err != nil {
		return err
	}
	unk = applyGitDigestHint(&inp, unk, n.digest)
	unk = applyHTTPDigestHint(&inp, unk, n.digest)
	inp.Env.Depth = depth
	n.input = inp
	n.unknowns = trimPrefixAll(unk, "input.")

	key := sourceKey(n.source, pl)
	if _, ok := stack[key]; ok {
		// Avoid cyclic material expansion for self-referential provenance graphs.
		n.children = nil
		if n.input.Image != nil && n.input.Image.Provenance != nil {
			n.input.Image.Provenance.Materials = nil
		}
		return nil
	}
	stack[key] = struct{}{}
	defer delete(stack, key)

	if n.input.Image == nil || n.input.Image.Provenance == nil || len(n.input.Image.Provenance.materialSources) == 0 {
		n.children = nil
		return nil
	}

	prev := n.children
	children := make([]*graphNode, 0, len(n.input.Image.Provenance.materialSources))
	materials := make([]Input, 0, len(n.input.Image.Provenance.materialSources))

	for i, m := range n.input.Image.Provenance.materialSources {
		var child *graphNode
		if i < len(prev) {
			p := prev[i]
			if p != nil && sourceEquals(p.source, m.Source) {
				child = p
			}
		}
			if child == nil {
				child = &graphNode{
					source:   cloneSourceOp(m.Source),
					platform: clonePlatform(m.Platform),
					digest:   maps.Clone(m.Digest),
					resp: &gwpb.ResolveSourceMetaResponse{
						Source: cloneSourceOp(m.Source),
					},
			}
		}
		child.path = joinMaterialPath(n.path, i)
		if child.source == nil {
			child.source = cloneSourceOp(m.Source)
		}
		if child.platform == nil {
			child.platform = clonePlatform(m.Platform)
		}
			if len(child.digest) == 0 {
				child.digest = maps.Clone(m.Digest)
			}
		if child.resp == nil {
			child.resp = &gwpb.ResolveSourceMetaResponse{Source: cloneSourceOp(child.source)}
		}
		if child.resp.Source == nil {
			child.resp.Source = cloneSourceOp(child.source)
		}
		if err := g.buildNode(ctx, child, depth+1, stack); err != nil {
			return err
		}
		children = append(children, child)
		materials = append(materials, child.input)
	}

	n.children = children
	n.input.Image.Provenance.Materials = materials
	return nil
}

func joinMaterialPath(parent string, idx int) string {
	seg := fmt.Sprintf("materials[%d]", idx)
	if parent == "" {
		return seg
	}
	return parent + "." + seg
}

// PlanResolveRequest maps unresolved input refs to one metadata request for a
// single source node in the input graph.
//
// Returns:
//   - targetPath: graph-relative material path where the response should be applied
//     ("" means root source)
//   - req: source metadata request for that target node
func (g *InputGraph) PlanResolveRequest(unknowns []string) (targetPath string, req *gwpb.ResolveSourceMetaRequest, err error) {
	type bucket struct {
		node *graphNode
		keys []string
	}
	buckets := map[string]*bucket{}
	order := []string{}

	for _, u := range unknowns {
		path := strings.TrimPrefix(u, "input.")
		node, key, routeErr := g.resolveTarget(path)
		if routeErr != nil || node == nil || key == "" {
			continue
		}
		if b, ok := buckets[node.path]; ok {
			if !slices.Contains(b.keys, key) {
				b.keys = append(b.keys, key)
			}
			continue
		}
		buckets[node.path] = &bucket{
			node: node,
			keys: []string{key},
		}
		order = append(order, node.path)
	}
	if len(order) == 0 {
		return "", nil, nil
	}
	b := buckets[order[0]]
	req = &gwpb.ResolveSourceMetaRequest{
		Source: cloneSourceOp(b.node.source),
	}
	reqPlatform := b.node.platform
	if reqPlatform == nil {
		reqPlatform = g.defaultPlatform
	}
	if reqPlatform != nil {
		req.Platform = &pb.Platform{
			OS:           reqPlatform.OS,
			Architecture: reqPlatform.Architecture,
			Variant:      reqPlatform.Variant,
		}
	}
	if err = AddUnknownsWithLogger(g.logf, req, b.keys); err != nil {
		return "", nil, err
	}
	return b.node.path, req, nil
}

func (g *InputGraph) resolveTarget(path string) (*graphNode, string, error) {
	node := g.root
	rem := strings.TrimPrefix(path, "input.")
	for strings.HasPrefix(rem, "image.provenance.materials[") {
		i, rest, ok := parseLeadingMaterialIndex(rem)
		if !ok {
			return nil, "", errors.Errorf("invalid material path %q", path)
		}
		if i < 0 || i >= len(node.children) {
			return nil, "", errors.Errorf("material path index %d out of bounds for %q", i, path)
		}
		node = node.children[i]
		rem = strings.TrimPrefix(rest, ".")
	}
	return node, rem, nil
}

func parseLeadingMaterialIndex(s string) (int, string, bool) {
	const prefix = "image.provenance.materials["
	if !strings.HasPrefix(s, prefix) {
		return 0, "", false
	}
	rest := strings.TrimPrefix(s, prefix)
	idxStr, tail, ok := strings.Cut(rest, "]")
	if !ok {
		return 0, "", false
	}
	idx, err := strconv.Atoi(idxStr)
	if err != nil {
		return 0, "", false
	}
	return idx, tail, true
}

func cloneSourceOp(src *pb.SourceOp) *pb.SourceOp {
	if src == nil {
		return nil
	}
	cp := &pb.SourceOp{
		Identifier: src.Identifier,
	}
	if len(src.Attrs) > 0 {
		cp.Attrs = make(map[string]string, len(src.Attrs))
		maps.Copy(cp.Attrs, src.Attrs)
	}
	return cp
}

func cloneSourceMetaResponse(src *gwpb.ResolveSourceMetaResponse) *gwpb.ResolveSourceMetaResponse {
	if src == nil {
		return nil
	}
	cp := &gwpb.ResolveSourceMetaResponse{
		Source: cloneSourceOp(src.Source),
		Image:  src.Image,
		Git:    src.Git,
		HTTP:   src.HTTP,
	}
	return cp
}

func sourceEquals(a, b *pb.SourceOp) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Identifier != b.Identifier {
		return false
	}
	if len(a.Attrs) != len(b.Attrs) {
		return false
	}
	for k, v := range a.Attrs {
		if b.Attrs[k] != v {
			return false
		}
	}
	return true
}

func sourceKey(src *pb.SourceOp, platform *ocispecs.Platform) string {
	if src == nil {
		return ""
	}
	p := ""
	if platform != nil {
		p = platforms.Format(*platform)
	}
	return src.Identifier + "|" + p
}

func clonePlatform(p *ocispecs.Platform) *ocispecs.Platform {
	if p == nil {
		return nil
	}
	cp := *p
	return &cp
}

func trimPrefixAll(fields []string, prefix string) []string {
	if len(fields) == 0 {
		return nil
	}
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimPrefix(field, prefix)
		if field == "" {
			continue
		}
		out = append(out, field)
	}
	return uniqueStrings(out)
}

func uniqueStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func applyGitDigestHint(inp *Input, unknowns []string, digest map[string]string) []string {
	if inp == nil || inp.Git == nil || len(digest) == 0 {
		return unknowns
	}
	hint := strings.TrimSpace(firstNonEmpty(digest["sha1"], digest["sha256"]))
	if hint == "" {
		return unknowns
	}
	inp.Git.Checksum = hint
	if len(hint) == 64 {
		inp.Git.IsSHA256 = true
	}

	unknowns = removeUnknown(unknowns, "input.git.checksum")
	unknowns = removeUnknown(unknowns, "input.git.isSHA256")
	return unknowns
}

func applyHTTPDigestHint(inp *Input, unknowns []string, digest map[string]string) []string {
	if inp == nil || inp.HTTP == nil || len(digest) == 0 {
		return unknowns
	}
	if inp.HTTP.Checksum != "" {
		return unknowns
	}
	algo := "sha256"
	val := strings.TrimSpace(digest[algo])
	if val == "" {
		return unknowns
	}
	inp.HTTP.Checksum = algo + ":" + val
	unknowns = removeUnknown(unknowns, "input.http.checksum")
	return unknowns
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func removeUnknown(in []string, key string) []string {
	out := in[:0]
	for _, v := range in {
		if v == key {
			continue
		}
		out = append(out, v)
	}
	return out
}
