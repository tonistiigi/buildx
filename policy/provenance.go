package policy

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	slsa02 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v0.2"
	slsa1 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v1"
	gwpb "github.com/moby/buildkit/frontend/gateway/pb"
	provenancetypes "github.com/moby/buildkit/solver/llbsolver/provenance/types"
	"github.com/sirupsen/logrus"
)

const predicateTypeAnnotation = "in-toto.io/predicate-type"

var resolveProvenanceAttestations = []string{
	slsa02.PredicateSLSAProvenance,
	slsa1.PredicateSLSAProvenance,
}

type inTotoStatement struct {
	PredicateType string          `json:"predicateType"`
	Predicate     json.RawMessage `json:"predicate"`
}

func parseProvenance(ac *gwpb.AttestationChain, logf func(logrus.Level, string)) (*ImageProvenance, error) {
	if ac == nil || len(ac.Blobs) == 0 {
		return nil, nil
	}

	// Prefer blobs that explicitly declare a provenance predicate type.
	for _, b := range ac.Blobs {
		if b == nil || b.Descriptor_ == nil || len(b.Data) == 0 {
			continue
		}
		pt := b.Descriptor_.Annotations[predicateTypeAnnotation]
		if strings.HasPrefix(pt, "https://slsa.dev/provenance/") {
			prv, err := parseProvenanceBlob(b.Data, pt, logf)
			if err != nil {
				return nil, err
			}
			if prv != nil {
				return prv, nil
			}
		}
	}

	// Fallback for blobs without descriptor annotations.
	for _, b := range ac.Blobs {
		if b == nil || len(b.Data) == 0 {
			continue
		}
		prv, err := parseProvenanceBlob(b.Data, "", logf)
		if err != nil {
			return nil, err
		}
		if prv != nil {
			return prv, nil
		}
	}

	return nil, nil
}

func parseProvenanceBlob(dt []byte, pt string, logf func(logrus.Level, string)) (*ImageProvenance, error) {
	switch pt {
	case slsa1.PredicateSLSAProvenance:
		return parseSLSA1Provenance(dt, logf)
	case slsa02.PredicateSLSAProvenance:
		return parseSLSA02Provenance(dt, logf)
	}

	var stmt inTotoStatement
	if err := json.Unmarshal(dt, &stmt); err == nil && stmt.PredicateType != "" && len(stmt.Predicate) > 0 {
		switch stmt.PredicateType {
		case slsa1.PredicateSLSAProvenance:
			return parseSLSA1Provenance(stmt.Predicate, logf)
		case slsa02.PredicateSLSAProvenance:
			return parseSLSA02Provenance(stmt.Predicate, logf)
		}
	}

	// Some BuildKit paths store raw predicate JSON without wrapping statement.
	if prv, err := parseSLSA1Provenance(dt, logf); err != nil {
		return nil, err
	} else if prv != nil {
		return prv, nil
	}
	return parseSLSA02Provenance(dt, logf)
}

func parseSLSA1Provenance(dt []byte, logf func(logrus.Level, string)) (*ImageProvenance, error) {
	var pred provenancetypes.ProvenancePredicateSLSA1
	if err := json.Unmarshal(dt, &pred); err != nil {
		return nil, nil
	}
	if pred.BuildDefinition.BuildType == "" && pred.RunDetails.Builder.ID == "" {
		return nil, nil
	}
	prv := &ImageProvenance{
		PredicateType: slsa1.PredicateSLSAProvenance,
		BuildType:     pred.BuildDefinition.BuildType,
		BuilderID:     pred.RunDetails.Builder.ID,
		ConfigSource: &ImageProvenanceConfigSource{
			URI:      pred.BuildDefinition.ExternalParameters.ConfigSource.URI,
			Checksum: pickChecksum(pred.BuildDefinition.ExternalParameters.ConfigSource.Digest),
			Path:     pred.BuildDefinition.ExternalParameters.ConfigSource.Path,
		},
		Frontend:  pred.BuildDefinition.ExternalParameters.Request.Frontend,
		BuildArgs: extractBuildArgs(pred.BuildDefinition.ExternalParameters.Request.Args),
		RawArgs:   pred.BuildDefinition.ExternalParameters.Request.Args,
	}
	prv.materialSources = materialSourcesFromSLSA1(pred.BuildDefinition.ResolvedDependencies, logf)

	if md := pred.RunDetails.Metadata; md != nil {
		prv.InvocationID = md.InvocationID
		prv.StartedOn = formatProvenanceTime(md.StartedOn)
		prv.FinishedOn = formatProvenanceTime(md.FinishedOn)
		prv.Reproducible = boolPtr(md.Reproducible)
		prv.Hermetic = boolPtr(md.Hermetic)
		prv.Completeness = &ImageProvenanceCompleteness{
			Parameters: boolPtr(md.Completeness.Request),
			Materials:  boolPtr(md.Completeness.ResolvedDependencies),
		}
	}

	return prv, nil
}

func parseSLSA02Provenance(dt []byte, logf func(logrus.Level, string)) (*ImageProvenance, error) {
	var pred provenancetypes.ProvenancePredicateSLSA02
	if err := json.Unmarshal(dt, &pred); err != nil {
		return nil, nil
	}
	if pred.BuildType == "" && pred.Builder.ID == "" {
		return nil, nil
	}
	prv := &ImageProvenance{
		PredicateType: slsa02.PredicateSLSAProvenance,
		BuildType:     pred.BuildType,
		BuilderID:     pred.Builder.ID,
		ConfigSource: &ImageProvenanceConfigSource{
			URI:      pred.Invocation.ConfigSource.URI,
			Checksum: pickChecksum(pred.Invocation.ConfigSource.Digest),
			Path:     pred.Invocation.ConfigSource.EntryPoint,
		},
		Frontend:  pred.Invocation.Parameters.Frontend,
		BuildArgs: extractBuildArgs(pred.Invocation.Parameters.Args),
		RawArgs:   pred.Invocation.Parameters.Args,
	}
	prv.materialSources = materialSourcesFromSLSA02(pred.Materials, logf)

	if md := pred.Metadata; md != nil {
		prv.InvocationID = md.BuildInvocationID
		prv.StartedOn = formatProvenanceTime(md.BuildStartedOn)
		prv.FinishedOn = formatProvenanceTime(md.BuildFinishedOn)
		prv.Reproducible = boolPtr(md.Reproducible)
		prv.Hermetic = boolPtr(md.Hermetic)
		prv.Completeness = &ImageProvenanceCompleteness{
			Parameters:  boolPtr(md.Completeness.Parameters),
			Environment: boolPtr(md.Completeness.Environment),
			Materials:   boolPtr(md.Completeness.Materials),
		}
	}

	return prv, nil
}

func boolPtr(v bool) *bool {
	return &v
}

func pickChecksum(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	for _, k := range []string{"sha256", "sha1"} {
		if v := strings.TrimSpace(m[k]); v != "" {
			if strings.Contains(v, ":") {
				return v
			}
			return k + ":" + v
		}
	}
	for k, v := range m {
		if v = strings.TrimSpace(v); v != "" {
			if strings.Contains(v, ":") {
				return v
			}
			if strings.TrimSpace(k) != "" {
				return k + ":" + v
			}
			return v
		}
	}
	return ""
}

func formatProvenanceTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func extractBuildArgs(args map[string]string) map[string]string {
	if len(args) == 0 {
		return nil
	}
	const prefix = "build-arg:"
	out := make(map[string]string)
	for k, v := range args {
		if name, ok := strings.CutPrefix(k, prefix); ok && name != "" {
			out[name] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func materialSourcesFromSLSA1(materials []slsa1.ResourceDescriptor, logf func(logrus.Level, string)) []materialSource {
	if len(materials) == 0 {
		return nil
	}
	out := make([]materialSource, 0, len(materials))
	for _, m := range materials {
		src, err := materialSourceFromURI(m.URI, m.Digest)
		if err != nil {
			if logf != nil {
				logf(logrus.WarnLevel, fmt.Sprintf("skipping unsupported provenance material %q: %v", m.URI, err))
			}
			continue
		}
		out = append(out, *src)
	}
	return out
}

func materialSourcesFromSLSA02(materials []slsa02.ProvenanceMaterial, logf func(logrus.Level, string)) []materialSource {
	if len(materials) == 0 {
		return nil
	}
	out := make([]materialSource, 0, len(materials))
	for _, m := range materials {
		src, err := materialSourceFromURI(m.URI, m.Digest)
		if err != nil {
			if logf != nil {
				logf(logrus.WarnLevel, fmt.Sprintf("skipping unsupported provenance material %q: %v", m.URI, err))
			}
			continue
		}
		out = append(out, *src)
	}
	return out
}
