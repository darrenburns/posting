
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

CHROME ?= /Applications/Google Chrome.app/Contents/MacOS/Google Chrome

.PHONY: readme-screenshot
# Re-capture the Posting 3 screenshot in the README. The window is the
# snapshot's 130x36 cells at 14px Fira Code (8.4x19.6px per cell).
readme-screenshot:
	@out=$$(mktemp -d) && README_SVG=$$out/readme.svg go test ./internal/ui -run TestGenerateReadmeScreenshot -count=1 \
		&& printf '<!doctype html><style>body{margin:0}img{display:block}</style><img src="readme.svg">' > $$out/index.html \
		&& "$(CHROME)" --headless=new --hide-scrollbars --force-device-scale-factor=2 --window-size=1092,706 \
			--virtual-time-budget=5000 --screenshot="$(CURDIR)/docs/assets/readme.png" "file://$$out/index.html" 2>/dev/null \
		&& rm -rf $$out
