# Third-party notices

Data that the server embeds or downloads, with its source and license. Go modules the server
depends on keep their own licenses in their own repositories.

## Embedded in the binary

`internal/analytics/lists/` holds lists that analytics matches requests against; `update.go` in that
directory fetches them again, and each file's first line names the commit it came from.

### Crawler user agents (`crawlers.txt`)

The `pattern` field of every entry of `crawler-user-agents.json` from
https://github.com/monperrus/crawler-user-agents, used to drop events from crawlers.

```
The MIT License (MIT)

Copyright (c) 2017 Martin Monperrus

Permission is hereby granted, free of charge, to any person obtaining a copy of
this software and associated documentation files (the "Software"), to deal in
the Software without restriction, including without limitation the rights to
use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
the Software, and to permit persons to whom the Software is furnished to do so,
subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
```

### Search engines, social networks and AI assistants (`sources.tsv`)

The names and hosts of `SearchEngines.yml`, `Socials.yml` and `AIAssistants.yml` from
https://github.com/matomo-org/searchengine-and-social-list, released under CC0 1.0 Universal
(https://creativecommons.org/publicdomain/zero/1.0/). Used to name a visit's source and pick its
channel. Yuva adds its own assistants and webmail hosts in `internal/analytics/sources.go`.

### Referrer spam (`spammers.txt`)

`spammers.txt` from https://github.com/matomo-org/referrer-spam-list, in the public domain (its
package declares CC0-1.0). Events whose referrer is on the list are dropped.

## Downloaded at run time

### DB-IP IP to City Lite

With `YUVA_GEOIP=dbip` (the default) each server downloads the month's IP to City Lite database from
https://db-ip.com/db/download/ip-to-city-lite into `YUVA_GEOIP_DIR`. It is not part of the source
or the image. The database is licensed under the Creative Commons Attribution 4.0 International
License (https://creativecommons.org/licenses/by/4.0/): pages that show locations from it must credit
"IP Geolocation by DB-IP" with a link to https://db-ip.com, which `GET
/v1/analytics/properties/{id}/health` returns as `geoip.attribution` and `geoip.attribution_url`.
