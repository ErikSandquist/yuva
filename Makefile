.PHONY: build web widget

build: web widget
	@if [ -f web/dist/index.html ]; then \
		find api/internal/ui/dist -mindepth 1 -delete && cp -R web/dist/. api/internal/ui/dist/; \
	fi
	@if [ -f sdk/js/dist/yuva.js ] && [ -f sdk/js/dist/yuva-chat.js ] && [ -f sdk/js/dist/yuva-docs.js ] && [ -f sdk/js/dist/yuva-subscribe.js ]; then \
		cp sdk/js/dist/yuva.js sdk/js/dist/yuva-chat.js sdk/js/dist/yuva-docs.js sdk/js/dist/yuva-subscribe.js api/internal/widget/dist/; \
		rm -rf api/internal/widget/dist/widget; \
		if [ -d sdk/js/dist/widget ]; then cp -R sdk/js/dist/widget api/internal/widget/dist/; fi; \
	fi
	@if [ -f sdk/js/dist/yuva-analytics.js ]; then cp sdk/js/dist/yuva-analytics.js api/internal/widget/dist/; fi
	@if [ -f sdk/js/dist/yuva-analytics-consent.js ]; then cp sdk/js/dist/yuva-analytics-consent.js api/internal/widget/dist/; fi
	cd api && CGO_ENABLED=0 go build -trimpath -o ../bin/yuva ./cmd/yuva

web:
	@if [ -f web/package.json ]; then \
		cd web && { [ -d node_modules ] || npm ci; } && npm run build; \
	fi

widget:
	@if [ -f sdk/js/package.json ]; then \
		cd sdk/js && { [ -d node_modules ] || bun install --frozen-lockfile; } && bun run build; \
	fi
