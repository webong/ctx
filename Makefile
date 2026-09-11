install:
	./install.sh
test:
	./bin/dctx status
	./bin/dctx ls | head -n 20
