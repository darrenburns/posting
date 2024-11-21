
.PHONY: test
test:
	$(run) pytest --cov=posting tests/ -n 24 -m "not serial" $(ARGS)
	$(run) pytest --cov-report term-missing --cov-append --cov=posting tests/ -m serial $(ARGS)

.PHONY: test-snapshot-update
test-snapshot-update:
	$(run) pytest --cov=posting tests/ -n 24 -m "not serial" --snapshot-update $(ARGS)
	$(run) pytest --cov-report term-missing --cov-append --cov=posting tests/ -m serial --snapshot-update $(ARGS)


.PHONY: test-ci
test-ci:
	$(run) pytest --cov=posting tests/ --cov-report term-missing $(ARGS)

.PHONY: build
build:
	$(run) pyinstaller -n posting -F --collect-all textual --collect-all posting  src/posting/__main__.py

.PHONY: clean
clean:
	rm -f *.spec
	rm -rf build
	rm -rf dist
