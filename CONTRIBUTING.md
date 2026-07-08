# Contribuir a smuf

¡Gracias por tu interés en contribuir! Antes de mandar un PR grande, abre un issue para discutir el cambio.

## Setup

```bash
git clone https://github.com/cdrusu/smuf.git
cd smuf
go build ./...
go test ./...
```

Requiere Go 1.25+.

## Flujo de trabajo

1. Haz fork del repo y crea una rama a partir de `main`.
2. Escribe tests para el código nuevo o modificado.
3. Asegúrate de que `go build ./...`, `go vet ./...` y `go test ./...` pasan sin errores.
4. Abre el pull request describiendo el problema que resuelve y cómo lo probaste.

## Estilo de código

- Sigue el formato estándar de Go (`gofmt`).
- Los comentarios y mensajes de commit pueden ir en español o inglés; sé consistente dentro de un mismo archivo.
- Evita añadir dependencias nuevas salvo que sean claramente necesarias.

## Reportar bugs

Abre un [issue](../../issues/new) con:
- Pasos para reproducir
- Comportamiento esperado vs. observado
- Versión de smuf y sistema operativo
