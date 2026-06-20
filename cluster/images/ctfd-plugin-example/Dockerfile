# A CTFd plugin packaged as an OCI image. The image filesystem IS the plugin
# directory: mounted at /opt/CTFd/CTFd/plugins/<name> via a Kubernetes Image
# Volume, CTFd picks it up at startup. Build for the cluster's arch and push to
# a registry, then reference it from the chart's `plugins[].image`.
FROM scratch
COPY __init__.py /__init__.py
