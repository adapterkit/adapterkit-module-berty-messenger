test: generate
	go mod tidy
	go test -v .
	@echo "Done."

generate: messenger.pb.go
.PHONY: generate

messenger.pb.go: messenger.proto
	protoc --go_out=./ --go-grpc_out=./ $<

clean:
	rm -f messenger.pb.goa
