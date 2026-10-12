
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

.PHONY: docs-screens
# Re-capture the Posting 3 screens shown on the docs homepage.
docs-screens:
	@out=$$(mktemp -d) && HOMEPAGE_OUT=$$out go test ./internal/ui -run TestGenerateHomepageScenes -count=1 && python3 docs/scripts/home_screens.py $$out && rm -rf $$out

.PHONY: readme-demo
# Re-record the Posting 3 demo GIF in the README. See demo/record.sh.
readme-demo:
	demo/record.sh
