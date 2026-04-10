BIN := kraai

.PHONY: build run clean

build:
	go build -o $(BIN) ./cmd

run: build
	./$(BIN) --local $(ARGS)

clean:
	rm -f $(BIN)
