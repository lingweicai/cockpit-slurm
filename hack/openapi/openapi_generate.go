package models

// Regenerate Slurm REST API models from the local slurmrestd binary:
// Run:
//    go generate ./internal/models
//
//go:generate sh -c "slurmrestd -d v0.0.43 --generate-openapi-spec > slurm-25.05.7-openapi-v0.0.43.json"
//go:generate sh -c "GOPROXY=https://goproxy.cn,direct GOSUMDB=off go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1 --generate types --package models -o slurm_openapi.gen.go slurm-25.05.7-openapi-v0.0.43.json"
//go:generate gofmt -w slurm_openapi.gen.go
