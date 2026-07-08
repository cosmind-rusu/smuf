# Troubleshooting

Common issues and their solutions.

## Client can't connect to server

**Error:** `cannot reach smuf-server at ...`

1. Check the server is running: `systemctl status smuf-server`
2. Check the control port is accessible: `nc -zv yourdomain.com 7000`
3. Check firewall rules: `sudo ufw status`
4. Verify `SMUF_SERVER` is set correctly

## Authentication failed

**Error:** `el servidor pide contraseña` / `la contraseña no coincide`

1. Generate a new token: `openssl rand -hex 32`
2. Set `SMUF_AUTH_TOKEN` on **both** server and client
3. Restart the server after changing the token
4. Make sure there are no trailing spaces or quotes in the .env file

## Subdomain already in use

**Error:** `ese nombre de subdominio ya está en uso`

- Choose a different subdomain name
- Or don't use `--sub` and let smuf assign a random one
- The subdomain is freed when the tunnel closes

## Rate limited

**Error:** `el servidor dice que hay demasiadas conexiones desde tu IP`

- The server limits connections per IP (`SMUF_MAX_CONNS_PER_IP`, default: 5)
- Wait a moment and try again
- Increase the limit on the server if needed

## Tunnel connects but browser shows nothing

1. Make sure your local app is running: `curl http://localhost:3000`
2. Check if your app binds to `0.0.0.0` or `127.0.0.1` (either works with smuf)
3. Try a different port
4. Check the dashboard at `http://yourdomain.com:8080/` to see active tunnels

## WebSocket connections fail

- smuf supports WebSocket passthrough by default
- If using a reverse proxy (Nginx), ensure it has the correct proxy settings:

```nginx
proxy_http_version 1.1;
proxy_set_header Upgrade $http_upgrade;
proxy_set_header Connection "upgrade";
proxy_buffering off;
```

## Docker deployment issues

**Server won't start in Docker:**

1. Check logs: `docker compose logs smuf-server`
2. Ensure environment variables are set in `docker-compose.yml`
3. Ports 7000 and 8080 must be exposed and accessible

## HTTPS certificate errors

**Let's Encrypt not working:**

1. Ensure ports 80 and 443 are accessible from the internet
2. Verify DNS points to your server: `dig yourdomain.com`
3. Check `SMUF_ACME_EMAIL` is set
4. Check server logs for ACME errors

## Getting help

If you're still stuck:

1. Search [existing issues](https://github.com/cosmind-rusu/smuf/issues)
2. Open a [new issue](https://github.com/cosmind-rusu/smuf/issues/new)
3. Start a [discussion](https://github.com/cosmind-rusu/smuf/discussions)
