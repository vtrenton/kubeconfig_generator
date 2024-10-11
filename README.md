## KubeConfig Generator

This is a small go program that I use for testing the creation of new users for multi-tenant kubernetes applications.

This program takes in a small configuration yaml (defined in the `new-user-template.yaml`) as the primary argument and does the following:

- creates new roles
- creates new clusterroles
- creates new rolebindings
- creates new clusterrolebindings
- generates a kubeconfig for the user

You still need to generate your own x509 certs signed with the `client-ca.crt` and `client-ca.key` from the kubernetes server.

generating these keys is pretty straightforward with the `openssl` application:

```bash
# Generate a new client key
openssl genrsa -out client.key 2048

# Create a CSR - make sure to set the CN to the user you are setting up
openssl req -new -key client.key -out client.csr -subj "/CN=example-user"

# sign the CSR 
openssl x509 -req -in client.csr -CA client-ca.crt -CAkey client-ca.key -CAcreateserial -out client.crt -days 365 -sha256
```

Then simply put the `client.crt` and `client.key` in the yaml config

# Building

you can build it locally with `go build cmd/kcgen/kecgen.go` or via the provided Dockerfile for a more consistent deployment.

```bash
# podman is the same command just s/docker/podman/g

docker build -t kcgen .
docker run --rm -v new-user-template.yaml:/app/new-user-template.yaml kcgen /app/my_template.yaml

```
