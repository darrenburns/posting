
.PHONY: test
test:
	$(run) pytest --cov=posting tests/ -vv -n 16 -m "not serial"
	$(run) pytest --cov-report term-missing --cov-append --cov=posting tests/ -vv -m serial

test-snapshot-update:
	$(run) pytest --cov=posting tests/ -vv -n 16 -m "not serial" --snapshot-update
	$(run) pytest --cov-report term-missing --cov-append --cov=posting tests/ -vv -m serial --snapshot-update