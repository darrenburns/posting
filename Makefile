
.PHONY: test
test:
	$(run) pytest --cov-report term-missing --cov=posting tests/ -vv -n 16