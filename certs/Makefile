all:
	openssl genrsa -out CA.key 4096
	openssl req -x509 -new -nodes -key CA.key -sha256 -days 3650 -out CA.crt
	openssl genrsa -out tls.key 4096
	openssl req -new -key tls.key -out tls.csr
	openssl x509 -req -in tls.csr -CA CA.crt -CAkey CA.key -CAcreateserial -out tls.crt -days 3650 -sha256
	rm tls.csr
