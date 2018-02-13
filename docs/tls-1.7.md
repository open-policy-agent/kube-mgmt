# Generating TLS Certificates (1.7)

External Admission Controllers must be secured with TLS. At a minimum you must:

- Provide the Kubernetes API server with a client key to use for
  webhook calls (`client.key` and `client.crt` below).

- Provide OPA with a server key so that the Kubernetes API server can
  authenticate it (`server.key` and `server.crt` below).

- Provide `kube-mgmt` with the CA certificate to register with the Kubernetes
  API server (`ca.crt` below).

Follow the steps below to generate the necessary files for test purposes.

First, generate create the required OpenSSL configuration files:

**client.conf**:

```
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth, serverAuth
subjectAltName = @alt_names
[alt_names]
IP.1 = 127.0.0.1
```

**server.conf**:

```
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth, serverAuth
subjectAltName = @alt_names
[alt_names]
IP.1 = 10.0.0.222
```

> The subjectAltName/IP address in the certificate MUST match the one configured
> on the Kubernetes Service.

Finally, generate the CA and client/server key pairs.

```bash
# Create a certificate authority
openssl genrsa -out ca.key 2048
openssl req -x509 -new -nodes -key ca.key -days 100000 -out ca.crt -subj "/CN=admission_ca"

# Create a server certiticate
openssl genrsa -out server.key 2048
openssl req -new -key server.key -out server.csr -subj "/CN=admission_server" -config server.conf
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out server.crt -days 100000 -extensions v3_req -extfile server.conf

# Create a client certiticate
openssl genrsa -out client.key 2048
openssl req -new -key client.key -out client.csr -subj "/CN=admission_client" -config client.conf
openssl x509 -req -in client.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out client.crt -days 100000 -extensions v3_req -extfile client.conf
```

If you are using minikube, you can specify the client TLS credentials with the following `minikube start` options:

```
--extra-config=apiserver.ProxyClientCertFile=/path/to/client.crt  # in VM
--extra-config=apiserver.ProxyClientKeyFile=/path/to/client.key   # in VM
```