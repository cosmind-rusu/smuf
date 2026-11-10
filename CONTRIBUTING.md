# Contribuir a smuf

Gracias por tu interés en contribuir. Antes de mandar un PR grande, abre un issue para discutir el problema y el enfoque.

## Requisitos y setup

Necesitas Go 1.26.3 o una versión posterior, igual que indica `go.mod`.

```bash
git clone https://github.com/cosmind-rusu/smuf.git
cd smuf
go build ./...
go vet ./...
go test ./...
```

Una compilación local sin las variables de versión de GoReleaser muestra `smuf dev` o `smuf-server dev` al usar `-v`. Es lo esperado.

## Flujo de trabajo

1. Haz fork del repositorio y crea una rama a partir de `main`.
2. Haz cambios acotados y añade o actualiza tests para el comportamiento modificado.
3. Ejecuta `go fmt ./...`, `go vet ./...`, `go test ./...` y `go build ./...`.
4. Si cambias configuración, protocolo, comandos o limitaciones visibles, actualiza también el README y las páginas de `docs/wiki` afectadas.
5. Abre el pull request describiendo el problema, el cambio y las pruebas ejecutadas. Indica cualquier caso que no hayas podido probar.

## Pruebas de comportamiento

El repositorio incluye tests unitarios y de integración en los paquetes Go. `go test ./...` debe cubrir los cambios normales de registro, límites y túneles.

También hay dos comprobaciones manuales con servicios locales:

```bash
go run ./cmd/test-tcp
go run ./cmd/test-websocket
```

Ejecuta la que corresponda cuando cambies el reenvío TCP, los upgrades HTTP o el ciclo de vida de las sesiones. No uses túneles públicos ni datos reales para estas pruebas.

## Estilo de código

- Usa el formato estándar de Go con `go fmt`.
- Los comentarios y mensajes de commit pueden ir en español o inglés; mantén un criterio consistente dentro de cada cambio.
- Evita añadir dependencias salvo que resuelvan una necesidad concreta que la biblioteca estándar y las dependencias actuales no cubran.
- No afirmes compatibilidad, rendimiento o seguridad sin una prueba reproducible o evidencia revisable.

## Documentación

Los materiales públicos deben reflejar el código, no una hoja de ruta. Al documentar una función, incluye sus valores predeterminados, requisitos de red y límites cuando sean relevantes. No uses cifras de usuarios o rendimiento, testimonios ni garantías sin una fuente verificable.

Para cambios de documentación, revisa los enlaces internos y ejecuta `git diff --check`. No hace falta añadir una herramienta nueva solo para validar Markdown.

## Reportar bugs

Abre un [issue](https://github.com/cosmind-rusu/smuf/issues/new) con:

- pasos para reproducir el problema;
- comportamiento esperado y observado;
- versión o commit de smuf, sistema operativo y arquitectura;
- configuración relevante y registros, con tokens, dominios privados, IP y otros datos sensibles eliminados.

Si el problema requiere una topología concreta, indica si usas binarios, Docker, un proxy inverso o una plataforma alojada, y qué puertos y nombres DNS llegan realmente a `smuf-server`.
