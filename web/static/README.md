# Static files

The server embeds these files. There is no frontend build or external asset request.

`htmx.min.js` is the unmodified HTMX 2.0.4 distribution from `https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js`, under the adjacent Zero-Clause BSD license. Only the search template loads it; HTMX only replaces search results. Every form and navigation link also works without JavaScript. Evaluation, script execution in responses, and local history caching are disabled.
