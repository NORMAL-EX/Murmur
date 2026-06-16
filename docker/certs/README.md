# Extra CA certificates (optional)

Put proxy / corporate root CA certificates here as PEM files ending in `.crt`
if you build the Docker image behind a TLS-intercepting proxy. They are trusted
by the build stages and the runtime image (certificate verification stays on).

Leave this directory empty (just `.gitkeep`) for normal builds — it is a no-op.
`*.crt` files here are git-ignored so private CAs are never committed.
