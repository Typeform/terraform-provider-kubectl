TEST?=$$(go list ./... |grep -v 'vendor')
GOFMT_FILES?=$$(find . -name '*.go' |grep -v vendor)

release:
	rm -fr bin
	mkdir -p bin
	GOARCH=amd64 GOOS=windows go build -o bin/terraform-provider-kubectl_windows_amd64.exe
	GOARCH=amd64 GOOS=linux go build -o bin/terraform-provider-kubectl_linux_amd64
	GOARCH=amd64 GOOS=darwin go build -o bin/terraform-provider-kubectl_darwin_amd64

build: fmtcheck
	go install

test: fmtcheck
	ginkgo -r || exit 1

testacc: fmtcheck
	TF_ACC=1 go test **/resource_*_test.go -v $(TESTARGS) -timeout 180m

fmtcheck:
	@sh -c "'$(CURDIR)/scripts/gofmtcheck.sh'"

fmt:
	gofmt -w $(GOFMT_FILES)

vet:
	@echo "go vet ."
	@go vet $$(go list ./... | grep -v vendor/) ; if [ $$? -eq 1 ]; then \
		echo ""; \
		echo "Vet found suspicious constructs. Please check the reported constructs"; \
		echo "and fix them if necessary before submitting the code for review."; \
		exit 1; \
	fi

vendor-status:
	@govendor status
