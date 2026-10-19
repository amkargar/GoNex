module github.com/amkargar/gonex/integration/rpc

go 1.27.1

require (
	github.com/amkargar/gonex v0.1.0
	github.com/amkargar/gonex/contracts v0.1.0
	google.golang.org/grpc v1.78.0
)

require (
	golang.org/x/net v0.53.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.36.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251029180050-ab9386a59fda // indirect
	google.golang.org/protobuf v1.36.10 // indirect
)

// Explicit local fixtures, never emitted into production service templates.
replace github.com/amkargar/gonex => ../..

replace github.com/amkargar/gonex/contracts => ../../contracts
