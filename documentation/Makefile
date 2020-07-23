serve:
	hugo server \
	--buildDrafts \
	--buildFuture \
	--disableFastRender \
	--ignoreCache

production-build:
	hugo

preview-build:
	hugo \
	--baseURL $(DEPLOY_PRIME_URL) \
	--buildDrafts \
	--buildFuture
