all:
	openssl genrsa -out CA.key 4096
	openssl req -x509 -new -nodes -key CA.key -sha256 -days 3650 -out CA.pem
	openssl genrsa -out server.key 4096
	openssl req -new -key server.key -out server.csr
	openssl x509 -req -in server.csr -CA CA.pem -CAkey CA.key -CAcreateserial -out server.pem -days 3650 -sha256
	rm server.csr
