target "default" {
  context = BAKE_CMD_CONTEXT
  dockerfile-inline = <<EOT
FROM alpine
WORKDIR /src
COPY . .
RUN ls -l && stop
EOT
}