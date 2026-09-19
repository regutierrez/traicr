# Keep traces inside the archive

Traicr stores unredacted work from repositories other people also use. Sharing, publishing, or exporting those traces would force an audience model and a redaction pipeline that the single-user archive does not have. Traicr therefore does not export, publish, share, or scrub traces for consumption outside the archive. The HTTP API exists for the collector and the browser, not for third parties.

This rejects the headline feature of traces.com and opentraces on purpose. A later change that adds any egress path must replace this decision, not quietly extend the API.
