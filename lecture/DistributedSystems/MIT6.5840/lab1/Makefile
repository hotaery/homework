WORKER_DIR=./worker
COORDINATOR_DIR=./coordinator
SEQUENTIAL_DIR=./sequential
DATA_DIR=./data
OUTPUT_DIR=./output
OUTPUT_BIN_DIR=$(OUTPUT_DIR)/bin
OUTPUT_LIB_DIR=$(OUTPUT_DIR)/lib
OUTPUT_DATA_DIR=$(OUTPUT_DIR)/data
MR_DIR=./mr
MRAPPS_DIR=./mrapps
PATTERN=*.go
RACE=-race

.PHONY: all

all: clean worker coordinator sequential mrapps data

deps:
	cd $(WORKER_DIR) && go mod tidy
	cd $(WORKER_DIR) && go mod download
	cd $(COORDINATOR_DIR) && go mod tidy
	cd $(COORDINATOR_DIR) && go mod download
	cd $(MR_DIR) && go mod tidy
	cd $(MR_DIR) && go mod download
	cd $(MRAPPS_DIR) && go mod tidy
	cd $(MRAPPS_DIR) && go mod download
	cd $(SEQUENTIAL_DIR) && go mod tidy
	cd $(SEQUENTIAL_DIR) && go mod download

prepare:
	mkdir $(OUTPUT_DIR)
	mkdir $(OUTPUT_BIN_DIR)
	mkdir $(OUTPUT_LIB_DIR)
	mkdir $(OUTPUT_DATA_DIR)

worker: prepare deps
	go build $(RACE) -o $(OUTPUT_BIN_DIR)/worker $(WORKER_DIR)

coordinator: prepare deps
	go build $(RACE) -o $(OUTPUT_BIN_DIR)/coordinator $(COORDINATOR_DIR)

sequential: prepare deps
	go build $(RACE) -o $(OUTPUT_BIN_DIR)/mrsequential $(SEQUENTIAL_DIR)

mrapps: prepare deps
	@for file in $(MRAPPS_DIR)/$(PATTERN); do \
		base=$$(basename $$file); \
		name=$${base%.*}; \
		go build $(RACE) -buildmode=plugin -o $(OUTPUT_LIB_DIR)/$$name.so $$file; \
	done

data: prepare
	cp $(DATA_DIR)/* $(OUTPUT_DATA_DIR)

run: worker coordinator mrapps data 
	cd $(OUTPUT_DIR) && bash test-mr.sh

clean:
	rm -rf $(OUTPUT_DIR)
