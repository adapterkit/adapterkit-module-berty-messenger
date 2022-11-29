module playground

go 1.18

replace berty.tech/berty/v2 => ../berty-clone

require (
	berty.tech/berty/v2 v2.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.47.0
	google.golang.org/protobuf v1.28.1
)

require (
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.1.0 // indirect
	github.com/gofrs/uuid v3.4.0+incompatible // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.16.0 // indirect
	github.com/klauspost/cpuid/v2 v2.1.1 // indirect
	github.com/libp2p/go-libp2p v0.23.3 // indirect
	github.com/libp2p/go-openssl v0.1.0 // indirect
	github.com/mattn/go-pointer v0.0.1 // indirect
	github.com/minio/sha256-simd v1.0.0 // indirect
	github.com/spacemonkeygo/spacelog v0.0.0-20180420211403-2296661a0572 // indirect
	golang.org/x/crypto v0.0.0-20220525230936-793ad666bf5e // indirect
	golang.org/x/net v0.0.0-20220920183852-bf014ff85ad5 // indirect
	golang.org/x/sys v0.0.0-20220915200043-7b5979e65e41 // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/xerrors v0.0.0-20220609144429-65e65417b02f // indirect
	google.golang.org/genproto v0.0.0-20211208223120-3a66f561d7aa // indirect
)

replace (
	bazil.org/fuse => bazil.org/fuse v0.0.0-20200117225306-7b5117fecadc // specific version for iOS building

	github.com/agl/ed25519 => github.com/agl/ed25519 v0.0.0-20170116200512-5312a6153412 // latest commit before the author shutdown the repo; see https://github.com/golang/go/issues/20504
	github.com/multiformats/go-multiaddr => github.com/gfanton/go-multiaddr v0.7.1-0.20221109002011-e39b3a49e793 // tmp, required for Android SDK30
	github.com/mutecomm/go-sqlcipher/v4 => github.com/berty/go-sqlcipher/v4 v4.4.3-0.20220810151512-74ea78235b48 // plaintext header support
	github.com/peterbourgon/ff/v3 => github.com/moul/ff/v3 v3.0.1 // temporary, see https://github.com/peterbourgon/ff/pull/67, https://github.com/peterbourgon/ff/issues/68
	golang.org/x/mobile => github.com/berty/mobile v0.0.8 // temporary, see https://github.com/golang/mobile/pull/58 and https://github.com/golang/mobile/pull/82
)
