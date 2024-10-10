package docker

import (
	"bytes"
	"context"

	"github.com/docker/buildx/util/progress"
	"github.com/pkg/errors"
)

func (d *Driver) createDevice(ctx context.Context, dev string, l progress.SubLogger) error {
	switch dev {
	case "docker.com/gpu":
		return d.createVenusGPU(ctx, l)
	default:
		return errors.Errorf("unsupported device %q, loading custom devices not implemented yet", dev)
	}
}

func (d *Driver) createVenusGPU(ctx context.Context, l progress.SubLogger) error {
	script := `#!/bin/sh
set -e
if [ ! -d /dev/dri ]; then
  echo >&2 "No Venus GPU detected. Requires Docker Desktop with Docker VMM virtualization enabled."
  exit 1
fi
mkdir -p /etc/cdi
cat <<EOF > /etc/cdi/venus-gpu.json
cdiVersion: "0.6.0"
kind: "docker.com/gpu"
annotations:
  cdi.device.name: "Virtio-GPU Venus (Docker Desktop)"
devices:
- name: venus
  containerEdits:
    deviceNodes:
    # make this dynamic
    - path: /dev/dri/card0
    - path: /dev/dri/renderD128
EOF
`

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	err := d.run(ctx, []string{"/bin/ash"}, bytes.NewReader([]byte(script)), stdout, stderr)
	if err != nil {
		l.Log(1, stdout.Bytes())
		l.Log(2, stderr.Bytes())
		return err
	}
	return nil
}
