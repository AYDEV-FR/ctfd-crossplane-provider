# Minimal CTFd plugin used to demonstrate installing a plugin from an OCI image
# via a Kubernetes Image Volume. CTFd calls load(app) for every plugin directory
# under CTFd/plugins at startup.


def load(app):
    app.logger.info("[example-plugin] loaded from an OCI image volume")
