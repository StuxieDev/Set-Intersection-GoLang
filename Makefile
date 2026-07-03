.PHONY: build test test-race vet run clean

build:
	go build -o set-intersection.exe ./cmd

test:
	go test ./... -v

test-race:
	go test -race ./... -v

vet:
	go vet ./...

run: build
	./set-intersection.exe -file1 examples/A_f.csv -file2 examples/B_f.csv -header -column udprn

clean:
	rm -f set-intersection.exe
