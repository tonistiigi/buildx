# buildx imagetools inspect

```
docker buildx imagetools inspect [OPTIONS] NAME
```

<!---MARKER_GEN_START-->
Show details of an image in the registry

### Options

| Name | Type | Default | Description |
| --- | --- | --- | --- |
| [`--builder`](#builder) | `string` |  | Override the configured builder instance |
| [`--format`](#format) | `string` | `{{.Manifest}}` | Format the output using the given Go template |
| [`--raw`](#raw) |  |  | Show original, unformatted JSON manifest |


<!---MARKER_GEN_END-->

## Description

Show details of an image in the registry.

```console
$ docker buildx imagetools inspect alpine
Name:      docker.io/library/alpine:latest
MediaType: application/vnd.docker.distribution.manifest.list.v2+json
Digest:    sha256:21a3deaa0d32a8057914f36584b5288d2e5ecc984380bc0118285c70fa8c9300

Manifests:
  Name:      docker.io/library/alpine:latest@sha256:e7d88de73db3d3fd9b2d63aa7f447a10fd0220b7cbf39803c803f2af9ba256b3
  MediaType: application/vnd.docker.distribution.manifest.v2+json
  Platform:  linux/amd64

  Name:      docker.io/library/alpine:latest@sha256:e047bc2af17934d38c5a7fa9f46d443f1de3a7675546402592ef805cfa929f9d
  MediaType: application/vnd.docker.distribution.manifest.v2+json
  Platform:  linux/arm/v6

  Name:      docker.io/library/alpine:latest@sha256:8483ecd016885d8dba70426fda133c30466f661bb041490d525658f1aac73822
  MediaType: application/vnd.docker.distribution.manifest.v2+json
  Platform:  linux/arm/v7

  Name:      docker.io/library/alpine:latest@sha256:c74f1b1166784193ea6c8f9440263b9be6cae07dfe35e32a5df7a31358ac2060
  MediaType: application/vnd.docker.distribution.manifest.v2+json
  Platform:  linux/arm64/v8

  Name:      docker.io/library/alpine:latest@sha256:2689e157117d2da668ad4699549e55eba1ceb79cb7862368b30919f0488213f4
  MediaType: application/vnd.docker.distribution.manifest.v2+json
  Platform:  linux/386

  Name:      docker.io/library/alpine:latest@sha256:2042a492bcdd847a01cd7f119cd48caa180da696ed2aedd085001a78664407d6
  MediaType: application/vnd.docker.distribution.manifest.v2+json
  Platform:  linux/ppc64le

  Name:      docker.io/library/alpine:latest@sha256:49e322ab6690e73a4909f787bcbdb873631264ff4a108cddfd9f9c249ba1d58e
  MediaType: application/vnd.docker.distribution.manifest.v2+json
  Platform:  linux/s390x
```

## Examples

### <a name="builder"></a> Override the configured builder instance (--builder)

Same as [`buildx --builder`](buildx.md#builder).

### <a name="format"></a> Format the output (--format)

Format the output using the given Go template. Defaults to `{{.Manifest}}` if
unset. Following fields are available:

* `.Name`: provides the reference of the image
* `.Manifest`: provides the manifest or manifest list
* `.Image`: provides the image config
* `.Provenance`: provides provenance or [build info from image config](https://github.com/moby/buildkit/blob/master/docs/build-repro.md#image-config)
* `.SBOM`: provides SBOM

#### `.Name`

```console
$ docker buildx imagetools inspect alpine --format "{{.Name}}"
Name: docker.io/library/alpine:latest
```

#### `.Manifest`

```console
$ docker buildx imagetools inspect crazymax/loop --format "{{.Manifest}}"
Name:      docker.io/crazymax/loop:latest
MediaType: application/vnd.docker.distribution.manifest.v2+json
Digest:    sha256:08602e7340970e92bde5e0a2e887c1fde4d9ae753d1e05efb4c8ef3b609f97f1
```

```console
$ docker buildx imagetools inspect moby/buildkit:master --format "{{.Manifest}}"
Name:      docker.io/moby/buildkit:master
MediaType: application/vnd.docker.distribution.manifest.list.v2+json
Digest:    sha256:3183f7ce54d1efb44c34b84f428ae10aaf141e553c6b52a7ff44cc7083a05a66

Manifests:
  Name:      docker.io/moby/buildkit:master@sha256:667d28c9fb33820ce686887a717a148e89fa77f9097f9352996bbcce99d352b1
  MediaType: application/vnd.docker.distribution.manifest.v2+json
  Platform:  linux/amd64

  Name:      docker.io/moby/buildkit:master@sha256:71789527b64ab3d7b3de01d364b449cd7f7a3da758218fbf73b9c9aae05a6775
  MediaType: application/vnd.docker.distribution.manifest.v2+json
  Platform:  linux/arm/v7

  Name:      docker.io/moby/buildkit:master@sha256:fb64667e1ce6ab0d05478f3a8402af07b27737598dcf9a510fb1d792b13a66be
  MediaType: application/vnd.docker.distribution.manifest.v2+json
  Platform:  linux/arm64

  Name:      docker.io/moby/buildkit:master@sha256:1c3ddf95a0788e23f72f25800c05abc4458946685e2b66788c3d978cde6da92b
  MediaType: application/vnd.docker.distribution.manifest.v2+json
  Platform:  linux/s390x

  Name:      docker.io/moby/buildkit:master@sha256:05bcde6d460a284e5bc88026cd070277e8380355de3126cbc8fe8a452708c6b1
  MediaType: application/vnd.docker.distribution.manifest.v2+json
  Platform:  linux/ppc64le

  Name:      docker.io/moby/buildkit:master@sha256:c04c57765304ab84f4f9807fff3e11605c3a60e16435c734b02c723680f6bd6e
  MediaType: application/vnd.docker.distribution.manifest.v2+json
  Platform:  linux/riscv64
```

#### `.Provenance`

```console
$ docker buildx imagetools inspect crazymax/buildx:buildinfo --format "{{.Provenance}}"
Name: docker.io/crazymax/buildx:buildinfo
BuildSource:
BuildDefinition: Dockerfile
BuildParameters:
  bar:     foo
  foo:     bar
Materials:
  Type: docker-image
  Ref:  docker.io/docker/buildx-bin:0.6.1@sha256:a652ced4a4141977c7daaed0a074dcd9844a78d7d2615465b12f433ae6dd29f0
  Pin:  sha256:a652ced4a4141977c7daaed0a074dcd9844a78d7d2615465b12f433ae6dd29f0

  Type: docker-image
  Ref:  docker.io/library/alpine:3.13
  Pin:  sha256:026f721af4cf2843e07bba648e158fb35ecc876d822130633cc49f707f0fc88c

  Type: docker-image
  Ref:  docker.io/moby/buildkit:v0.9.0
  Pin:  sha256:8dc668e7f66db1c044aadbed306020743516a94848793e0f81f94a087ee78cab

  Type: docker-image
  Ref:  docker.io/tonistiigi/xx@sha256:21a61be4744f6531cb5f33b0e6f40ede41fa3a1b8c82d5946178f80cc84bfc04
  Pin:  sha256:21a61be4744f6531cb5f33b0e6f40ede41fa3a1b8c82d5946178f80cc84bfc04

  Type: http
  Ref:  https://raw.githubusercontent.com/moby/moby/master/README.md
  Pin:  sha256:419455202b0ef97e480d7f8199b26a721a417818bc0e2d106975f74323f25e6c
```

#### JSON output

A `json` go template func is also available if you want to render fields as
JSON bytes:

```console
$ docker buildx imagetools inspect crazymax/loop --format "{{json .Manifest}}"
```
```json
{
  "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
  "digest": "sha256:08602e7340970e92bde5e0a2e887c1fde4d9ae753d1e05efb4c8ef3b609f97f1",
  "size": 949
}
```

```console
$ docker buildx imagetools inspect moby/buildkit:master --format "{{json .Manifest}}"
```
```json
{
  "schemaVersion": 2,
  "mediaType": "application/vnd.docker.distribution.manifest.list.v2+json",
  "digest": "sha256:eef5f92f1e942856995ae4714b85a58277b2a7fcc3bcb62ea2f0d38e0f5e88de",
  "size": 2010,
  "manifests": [
    {
      "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
      "digest": "sha256:f9f41c85124686c2afe330a985066748a91d7a5d505777fe274df804ab5e077e",
      "size": 1158,
      "platform": {
        "architecture": "amd64",
        "os": "linux"
      }
    },
    {
      "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
      "digest": "sha256:82097c2be19c617aafb3c3e43c88548738d4b2bf3db5c36666283a918b390266",
      "size": 1158,
      "platform": {
        "architecture": "arm",
        "os": "linux",
        "variant": "v7"
      }
    },
    {
      "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
      "digest": "sha256:b6b91e6c823d7220ded7d3b688e571ba800b13d91bbc904c1d8053593e3ee42c",
      "size": 1158,
      "platform": {
        "architecture": "arm64",
        "os": "linux"
      }
    },
    {
      "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
      "digest": "sha256:797061bcc16778de048b96f769c018ec24da221088050bbe926ea3b8d51d77e8",
      "size": 1158,
      "platform": {
        "architecture": "s390x",
        "os": "linux"
      }
    },
    {
      "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
      "digest": "sha256:b93d3a84d18c4d0b8c279e77343d854d9b5177df7ea55cf468d461aa2523364e",
      "size": 1159,
      "platform": {
        "architecture": "ppc64le",
        "os": "linux"
      }
    },
    {
      "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
      "digest": "sha256:d5c950dd1b270d437c838187112a0cb44c9258248d7a3a8bcb42fae8f717dc01",
      "size": 1158,
      "platform": {
        "architecture": "riscv64",
        "os": "linux"
      }
    }
  ]
}
```

```console
$ docker buildx imagetools inspect crazymax/buildkit:attest --format "{{json .Provenance}}"
```
```json
{
  "Materials": [
    {
      "Type": "docker-image",
      "Ref": "docker.io/docker/buildkit-syft-scanner:stable-1",
      "Pin": "sha256:b45f1d207e16c3a3a5a10b254ad8ad358d01f7ea090d382b95c6b2ee2b3ef765"
    },
    {
      "Type": "docker-image",
      "Ref": "docker.io/library/alpine:latest",
      "Pin": "sha256:8914eb54f968791faf6a8638949e480fef81e697984fba772b3976835194c6d4"
    }
  ]
}
```

```console
$ docker buildx imagetools inspect crazymax/buildx:buildinfo --format "{{json .}}"
```
```json
{
  "name": "crazymax/buildx:buildinfo",
  "manifest": {
    "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
    "digest": "sha256:899d2c7acbc124d406820857bb51d9089717bbe4e22b97eb4bc5789e99f09f83",
    "size": 2628
  },
  "image": {
    "created": "2022-02-24T12:27:43.627154558Z",
    "architecture": "amd64",
    "os": "linux",
    "config": {
      "Env": [
        "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
        "DOCKER_TLS_CERTDIR=/certs",
        "DOCKER_CLI_EXPERIMENTAL=enabled"
      ],
      "Entrypoint": [
        "docker-entrypoint.sh"
      ],
      "Cmd": [
        "sh"
      ]
    },
    "rootfs": {
      "type": "layers",
      "diff_ids": [
        "sha256:7fcb75871b2101082203959c83514ac8a9f4ecfee77a0fe9aa73bbe56afdf1b4",
        "sha256:d3c0b963ff5684160641f936d6a4aa14efc8ff27b6edac255c07f2d03ff92e82",
        "sha256:3f8d78f13fa9b1f35d3bc3f1351d03a027c38018c37baca73f93eecdea17f244",
        "sha256:8e6eb1137b182ae0c3f5d40ca46341fda2eaeeeb5fa516a9a2bf96171238e2e0",
        "sha256:fde4c869a56b54dd76d7352ddaa813fd96202bda30b9dceb2c2f2ad22fa2e6ce",
        "sha256:52025823edb284321af7846419899234b3c66219bf06061692b709875ed0760f",
        "sha256:50adb5982dbf6126c7cf279ac3181d1e39fc9116b610b947a3dadae6f7e7c5bc",
        "sha256:9801c319e1c66c5d295e78b2d3e80547e73c7e3c63a4b71e97c8ca357224af24",
        "sha256:dfbfac44d5d228c49b42194c8a2f470abd6916d072f612a6fb14318e94fde8ae",
        "sha256:3dfb74e19dedf61568b917c19b0fd3ee4580870027ca0b6054baf239855d1322",
        "sha256:b182e707c23e4f19be73f9022a99d2d1ca7bf1ca8f280d40e4d1c10a6f51550e"
      ]
    },
    "history": [
      {
        "created": "2021-11-12T17:19:58.698676655Z",
        "created_by": "/bin/sh -c #(nop) ADD file:5a707b9d6cb5fff532e4c2141bc35707593f21da5528c9e71ae2ddb6ba4a4eb6 in / "
      },
      {
        "created": "2021-11-12T17:19:58.948920855Z",
        "created_by": "/bin/sh -c #(nop)  CMD [\"/bin/sh\"]",
        "empty_layer": true
      },
      {
        "created": "2022-02-24T12:27:38.285594601Z",
        "created_by": "RUN /bin/sh -c apk --update --no-cache add     bash     ca-certificates     openssh-client   \u0026\u0026 rm -rf /tmp/* /var/cache/apk/* # buildkit",
        "comment": "buildkit.dockerfile.v0"
      },
      {
        "created": "2022-02-24T12:27:41.061874167Z",
        "created_by": "COPY /opt/docker/ /usr/local/bin/ # buildkit",
        "comment": "buildkit.dockerfile.v0"
      },
      {
        "created": "2022-02-24T12:27:41.174098947Z",
        "created_by": "COPY /usr/bin/buildctl /usr/local/bin/buildctl # buildkit",
        "comment": "buildkit.dockerfile.v0"
      },
      {
        "created": "2022-02-24T12:27:41.320343683Z",
        "created_by": "COPY /usr/bin/buildkit* /usr/local/bin/ # buildkit",
        "comment": "buildkit.dockerfile.v0"
      },
      {
        "created": "2022-02-24T12:27:41.447149933Z",
        "created_by": "COPY /buildx /usr/libexec/docker/cli-plugins/docker-buildx # buildkit",
        "comment": "buildkit.dockerfile.v0"
      },
      {
        "created": "2022-02-24T12:27:43.057722191Z",
        "created_by": "COPY /opt/docker-compose /usr/libexec/docker/cli-plugins/docker-compose # buildkit",
        "comment": "buildkit.dockerfile.v0"
      },
      {
        "created": "2022-02-24T12:27:43.145224134Z",
        "created_by": "ADD https://raw.githubusercontent.com/moby/moby/master/README.md / # buildkit",
        "comment": "buildkit.dockerfile.v0"
      },
      {
        "created": "2022-02-24T12:27:43.422212427Z",
        "created_by": "ENV DOCKER_TLS_CERTDIR=/certs",
        "comment": "buildkit.dockerfile.v0",
        "empty_layer": true
      },
      {
        "created": "2022-02-24T12:27:43.422212427Z",
        "created_by": "ENV DOCKER_CLI_EXPERIMENTAL=enabled",
        "comment": "buildkit.dockerfile.v0",
        "empty_layer": true
      },
      {
        "created": "2022-02-24T12:27:43.422212427Z",
        "created_by": "RUN /bin/sh -c docker --version   \u0026\u0026 buildkitd --version   \u0026\u0026 buildctl --version   \u0026\u0026 docker buildx version   \u0026\u0026 docker compose version   \u0026\u0026 mkdir /certs /certs/client   \u0026\u0026 chmod 1777 /certs /certs/client # buildkit",
        "comment": "buildkit.dockerfile.v0"
      },
      {
        "created": "2022-02-24T12:27:43.514320155Z",
        "created_by": "COPY rootfs/modprobe.sh /usr/local/bin/modprobe # buildkit",
        "comment": "buildkit.dockerfile.v0"
      },
      {
        "created": "2022-02-24T12:27:43.627154558Z",
        "created_by": "COPY rootfs/docker-entrypoint.sh /usr/local/bin/ # buildkit",
        "comment": "buildkit.dockerfile.v0"
      },
      {
        "created": "2022-02-24T12:27:43.627154558Z",
        "created_by": "ENTRYPOINT [\"docker-entrypoint.sh\"]",
        "comment": "buildkit.dockerfile.v0",
        "empty_layer": true
      },
      {
        "created": "2022-02-24T12:27:43.627154558Z",
        "created_by": "CMD [\"sh\"]",
        "comment": "buildkit.dockerfile.v0",
        "empty_layer": true
      }
    ]
  },
  "provenance": {
    "BuildDefinition": "Dockerfile",
    "BuildParameters": {
      "bar": "foo",
      "foo": "bar"
    },
    "Materials": [
      {
        "Type": "docker-image",
        "Ref": "docker.io/docker/buildx-bin:0.6.1@sha256:a652ced4a4141977c7daaed0a074dcd9844a78d7d2615465b12f433ae6dd29f0",
        "Pin": "sha256:a652ced4a4141977c7daaed0a074dcd9844a78d7d2615465b12f433ae6dd29f0"
      },
      {
        "Type": "docker-image",
        "Ref": "docker.io/library/alpine:3.13",
        "Pin": "sha256:026f721af4cf2843e07bba648e158fb35ecc876d822130633cc49f707f0fc88c"
      },
      {
        "Type": "docker-image",
        "Ref": "docker.io/moby/buildkit:v0.9.0",
        "Pin": "sha256:8dc668e7f66db1c044aadbed306020743516a94848793e0f81f94a087ee78cab"
      },
      {
        "Type": "docker-image",
        "Ref": "docker.io/tonistiigi/xx@sha256:21a61be4744f6531cb5f33b0e6f40ede41fa3a1b8c82d5946178f80cc84bfc04",
        "Pin": "sha256:21a61be4744f6531cb5f33b0e6f40ede41fa3a1b8c82d5946178f80cc84bfc04"
      },
      {
        "Type": "http",
        "Ref": "https://raw.githubusercontent.com/moby/moby/master/README.md",
        "Pin": "sha256:419455202b0ef97e480d7f8199b26a721a417818bc0e2d106975f74323f25e6c"
      }
    ]
  }
}
```

#### Multi-platform

Multi-platform images are supported for `.Image`, `.Provenance` and `.SBOM`
fields. If you want to pick up a specific platform, you can specify it using
the `index` go template function:

```console
$ docker buildx imagetools inspect --format '{{json (index .Image "linux/s390x")}}' moby/buildkit:master
```
```json
{
  "created": "2022-11-30T17:42:26.414957336Z",
  "architecture": "s390x",
  "os": "linux",
  "config": {
    "Env": [
      "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
    ],
    "Entrypoint": [
      "buildkitd"
    ],
    "Volumes": {
      "/var/lib/buildkit": {}
    }
  },
  "rootfs": {
    "type": "layers",
    "diff_ids": [
      "sha256:41048e32d0684349141cf05f629c5fc3c5915d1f3426b66dbb8953a540e01e1e",
      "sha256:2651209b9208fff6c053bc3c17353cb07874e50f1a9bc96d6afd03aef63de76a",
      "sha256:88577322e65f094ce8ac27435880f1a8a9baadb569258026bb141770451bafcb",
      "sha256:de8f9a790e4ed10ff1f1f8ea923c9da4f97246a7e200add2dc6650eba3f10a20"
    ]
  },
  "history": [
    {
      "created": "2021-11-24T20:41:23.709681315Z",
      "created_by": "/bin/sh -c #(nop) ADD file:cd24c711a2ef431b3ff94f9a02bfc42f159bc60de1d0eceecafea4e8af02441d in / "
    },
    {
      "created": "2021-11-24T20:41:23.94211262Z",
      "created_by": "/bin/sh -c #(nop)  CMD [\"/bin/sh\"]",
      "empty_layer": true
    },
    {
      "created": "2022-01-26T18:15:21.449825391Z",
      "created_by": "RUN /bin/sh -c apk add --no-cache fuse3 git openssh pigz xz   \u0026\u0026 ln -s fusermount3 /usr/bin/fusermount # buildkit",
      "comment": "buildkit.dockerfile.v0"
    },
    {
      "created": "2022-08-25T00:39:25.652811078Z",
      "created_by": "COPY examples/buildctl-daemonless/buildctl-daemonless.sh /usr/bin/ # buildkit",
      "comment": "buildkit.dockerfile.v0"
    },
    {
      "created": "2022-11-30T17:42:26.414957336Z",
      "created_by": "VOLUME [/var/lib/buildkit]",
      "comment": "buildkit.dockerfile.v0",
      "empty_layer": true
    },
    {
      "created": "2022-11-30T17:42:26.414957336Z",
      "created_by": "COPY / /usr/bin/ # buildkit",
      "comment": "buildkit.dockerfile.v0"
    },
    {
      "created": "2022-11-30T17:42:26.414957336Z",
      "created_by": "ENTRYPOINT [\"buildkitd\"]",
      "comment": "buildkit.dockerfile.v0",
      "empty_layer": true
    }
  ]
}
```

### <a name="raw"></a> Show original, unformatted JSON manifest (--raw)

Use the `--raw` option to print the unformatted JSON manifest bytes.

> `jq` is used here to get a better rendering of the output result.

```console
$ docker buildx imagetools inspect --raw crazymax/loop | jq
```
```json
{
  "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
  "schemaVersion": 2,
  "config": {
    "mediaType": "application/vnd.docker.container.image.v1+json",
    "digest": "sha256:a98999183d2c7a8845f6d56496e51099ce6e4359ee7255504174b05430c4b78b",
    "size": 2762
  },
  "layers": [
    {
      "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
      "digest": "sha256:8663204ce13b2961da55026a2034abb9e5afaaccf6a9cfb44ad71406dcd07c7b",
      "size": 2818370
    },
    {
      "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
      "digest": "sha256:f0868a92f8e1e5018ed4e60eb845ed4ff0e2229897f4105e5a4735c1d6fd874f",
      "size": 1821402
    },
    {
      "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
      "digest": "sha256:d010066dbdfcf7c12fca30cd4b567aa7218eb6762ab53169d043655b7a8d7f2e",
      "size": 404457
    }
  ]
}
```

```console
$ docker buildx imagetools inspect --raw moby/buildkit:master | jq
```
```json
{
  "mediaType": "application/vnd.docker.distribution.manifest.list.v2+json",
  "schemaVersion": 2,
  "manifests": [
    {
      "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
      "digest": "sha256:f9f41c85124686c2afe330a985066748a91d7a5d505777fe274df804ab5e077e",
      "size": 1158,
      "platform": {
        "architecture": "amd64",
        "os": "linux"
      }
    },
    {
      "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
      "digest": "sha256:82097c2be19c617aafb3c3e43c88548738d4b2bf3db5c36666283a918b390266",
      "size": 1158,
      "platform": {
        "architecture": "arm",
        "os": "linux",
        "variant": "v7"
      }
    },
    {
      "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
      "digest": "sha256:b6b91e6c823d7220ded7d3b688e571ba800b13d91bbc904c1d8053593e3ee42c",
      "size": 1158,
      "platform": {
        "architecture": "arm64",
        "os": "linux"
      }
    },
    {
      "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
      "digest": "sha256:797061bcc16778de048b96f769c018ec24da221088050bbe926ea3b8d51d77e8",
      "size": 1158,
      "platform": {
        "architecture": "s390x",
        "os": "linux"
      }
    },
    {
      "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
      "digest": "sha256:b93d3a84d18c4d0b8c279e77343d854d9b5177df7ea55cf468d461aa2523364e",
      "size": 1159,
      "platform": {
        "architecture": "ppc64le",
        "os": "linux"
      }
    },
    {
      "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
      "digest": "sha256:d5c950dd1b270d437c838187112a0cb44c9258248d7a3a8bcb42fae8f717dc01",
      "size": 1158,
      "platform": {
        "architecture": "riscv64",
        "os": "linux"
      }
    }
  ]
}
```
