module github.com/novikovoleg142/vault_raft_snapshot_agent

go 1.13

require (
	cloud.google.com/go v0.38.0
	github.com/Azure/azure-storage-blob-go v0.8.0
	github.com/Azure/go-autorest/autorest/adal v0.8.3 // indirect
	github.com/ProtonMail/gopenpgp/v2 v2.0.1
	github.com/aws/aws-sdk-go v1.30.14
	github.com/hashicorp/vault/api v1.0.4
	go.opencensus.io v0.22.3 // indirect
	golang.org/x/crypto v0.0.0-20191206172530-e9b2fee46413 // indirect
	google.golang.org/api v0.22.0
	google.golang.org/grpc v1.27.0 // indirect
)

replace golang.org/x/crypto => github.com/ProtonMail/crypto v0.0.0-20190427044656-efb430e751f2
