
.PHONY: test
test:
	$(run) pytest --cov=posting tests/ -n 16 -m "not serial" $(ARGS)
	$(run) pytest --cov-report term-missing --cov-append --cov=posting tests/ -m serial $(ARGS)

test-snapshot-update:
	$(run) pytest --cov=posting tests/ -n 16 -m "not serial" --snapshot-update $(ARGS)
	$(run) pytest --cov-report term-missing --cov-append --cov=posting tests/ -m serial --snapshot-update $(ARGS)