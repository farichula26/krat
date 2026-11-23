# Carta en Redis

## Context
Las tools de la carta consultaban el Postgres remoto en cada pregunta, a través del túnel SSH:
- unos 300 ms de ping,
- unos 6 s para abrir el SSH,
- un timeout de 5 s.

Eso hacía lento a krat, y cuando el túnel se caía se quedaba sin carta. Farid pidió un Redis local en podman (red host, datos en `data/redis/`) que se actualice desde el Postgres y que el agente consulte directamente.

## Approaches (considerados)
1. **Carta completa como un JSON en Redis, con la búsqueda en Go ← elegido.**
   - Son unos 356 productos, así que la carta cabe entera en una clave.
   - Las tools leen la clave en menos de 1 ms y filtran y ordenan en Go, con las mismas reglas que antes tenía el SQL.
2. **RediSearch (`FT.SEARCH`).** Habría que reproducir el tratamiento de tildes y el orden por coincidencias en su sintaxis. Con este tamaño de carta no gana velocidad.
3. **Un proceso de sync aparte.** Sería otra pieza que arrancar y vigilar, con su propio túnel.

## Decisiones
- **Sin cambios en el frontend.**
- **Sync desde el backend:** cada 60 s, con `cartaSync` en `carta.go`.
  - Una sola consulta.
  - No sobrescribe la copia si la consulta falla o devuelve 0 filas.
- **Persistencia:** Redis con AOF en `data/redis/`. Si el Postgres no está, krat sigue respondiendo con la última copia, incluso después de reiniciar el backend.
- **Seguridad:** Redis escucha solo en `127.0.0.1`, porque con la red host no tiene contraseña. `--userns=keep-id` deja los archivos a nombre del usuario.
- **Formato de los resultados:** igual que antes (`sucursal, nombre, categoria, descripcion, precio`; precio `null` si no hay). Para Gemini no cambia nada.
- **Log de cada tool:** incluye la duración, `→ n filas (1.2ms)`.
