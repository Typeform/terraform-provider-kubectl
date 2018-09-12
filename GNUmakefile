TEST?=$$(go list ./... |grep -v 'vendor')
GOFMT_FILES?=$$(find . -name '*.go' |grep -v vendor)

test: fmtcheck
	ginkgo -r || exit 1

testacc: fmtcheck
	TF_ACC=1 go test **/resource_*_test.go -v $(TESTARGS) -timeout 180m

fmtcheck:
	@sh -c "'$(CURDIR)/scripts/gofmtcheck.sh'"

fmt:
	gofmt -w $(GOFMT_FILES)

vendor-status:
	@govendor status
