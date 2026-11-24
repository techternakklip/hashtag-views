.PHONY: dev dev-backend dev-frontend build test vet fetch report export clean

HASHTAG ?= gta
PLATFORMS ?= ig,fb
LIMIT ?= 50
FORMAT ?= csv
OUT ?= ./exports
MOCK_FLAG := $(if $(filter 1 true,$(MOCK)),--mock,)
ENRICH_FLAG := $(if $(filter 1 true,$(ENRICH_FB)),--enrich-fb,)

# Run backend (:8080) and frontend (:5173) together; Ctrl-C stops both.
dev:
	$(MAKE) -j2 dev-backend dev-frontend

dev-backend:
	cd backend && go run ./cmd/meta-views serve

dev-frontend:
	cd frontend && npm run dev

build:
	cd frontend && npm run build
	cd backend && go build -o ../bin/meta-views ./cmd/meta-views

test:
	cd backend && go test -v -race ./...
	cd frontend && npx tsc --noEmit && npm run build

vet:
	cd backend && go vet ./...

fetch:
	cd backend && go run ./cmd/meta-views fetch --hashtag "$(HASHTAG)" --platforms "$(PLATFORMS)" --limit $(LIMIT) $(MOCK_FLAG) $(ENRICH_FLAG)

report:
	cd backend && go run ./cmd/meta-views report --hashtag "$(HASHTAG)"

export:
	cd backend && go run ./cmd/meta-views export --hashtag "$(HASHTAG)" --format "$(FORMAT)" --out "../$(OUT)"

clean:
	rm -rf bin backend/meta-views frontend/dist
