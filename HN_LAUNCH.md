# Show HN: smuf, materiales de lanzamiento

## Título

```text
Show HN: smuf, a self-hosted HTTP/TCP tunnel written in Go
```

Alternativa más centrada en el caso de uso:

```text
Show HN: smuf, expose localhost through your own server and domain
```

## Primer comentario

```text
Hey HN! I built smuf, a small self-hosted tunnel client and server in Go. It
exposes a local HTTP service on a subdomain of your own domain, or a local TCP
service on a public port allocated by your server.

I wanted a direct setup without a separate hosted control plane. You run
smuf-server on a reachable host, run smuf next to the local application, and
connect the two with a shared token. Each tunnel uses yamux to multiplex
streams over one TCP connection.

What it does today:
• HTTP and raw TCP tunnels
• Random 12-character HTTP subdomain IDs, or a requested fixed subdomain
• Multiple local ports from one client process
• WebSocket, SSE, long-polling, and streaming HTTP passthrough
• Automatic public HTTPS with Let's Encrypt
• Per-IP control connection and connection-attempt limits
• A dashboard and JSON endpoint showing active tunnel metadata
• An interactive setup wizard and environment-based configuration

What it does not do yet:
• UDP
• TLS on the client-to-server control channel
• Authentication for public visitors or for the dashboard
• Persistent or shared tunnel state across server replicas
• Server-wide connection, stream, or resource limits

That last set matters if you want to expose something sensitive. The token is
sent in the control handshake over raw TCP, and HTTPS only protects the
visitor-to-server hop. The ACME policy also accepts arbitrary subdomains through
SNI, and the HTTPS redirect trusts Host. Those paths need hardening or a trusted
TLS proxy. For now I recommend demos or protected personal use, not a public
multi-tenant service. I have tried to make those boundaries explicit in the
README rather than imply that self-hosting alone makes an exposed app safe.

The source is Apache 2.0:
  https://github.com/cosmind-rusu/smuf

The published v0.4.0 release includes Linux amd64/arm64, macOS amd64/arm64,
and Windows amd64 archives, plus checksums.txt. It is not a draft or prerelease.

Build with Go 1.26.3 or newer:
  go build -o smuf-server ./cmd/smuf-server
  go build -o smuf ./cmd/smuf

Then run ./smuf-server --setup and ./smuf --setup, or use the environment
variables in the README. A normal HTTP tunnel looks like:
  ./smuf 3000

The repository includes Docker and Railway starting points. Docker Compose
needs your domain and token, and its TCP range needs additional port mappings.
Railway still needs verification of raw TCP access and wildcard subdomain
routing, so I am not presenting it as a turnkey deployment.

I would especially like feedback on the control-channel security model, the
configuration behavior, and whether the limitations are clear. What would you
need before using this for your own development tunnels?
```

## Antes de publicar

1. Comprueba que el enlace del repositorio abre la versión que quieres mostrar y que las instrucciones del README funcionan desde un entorno limpio.
2. La release `v0.4.0` y sus archives para Linux, macOS y Windows, junto con `checksums.txt`, ya están verificados. Para cualquier otra versión, comprueba los artifacts antes de anunciarlos: GoReleaser crea borradores y una etiqueta no prueba publicación. No anuncies una imagen oficial, compatibilidad con Railway o estado de producción sin verificarlos.
3. Si cambia el protocolo o la seguridad, actualiza el comentario y el README a la vez. No uses frases como «sin límites», «cero configuración» o «siempre disponible».
4. Responde con detalles técnicos y reconoce las limitaciones cuando alguien las señale. No inventes usuarios, métricas, pruebas de carga ni testimonios para defender el proyecto.
5. Conserva las preguntas concretas del final. Sirven para pedir revisión sobre decisiones reales, no solo para pedir valoración genérica.
