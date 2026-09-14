---
kind: decision
state: resolved
column: 4b38abe7ebd5
owner: holder
ts: 2026-09-14T02:18:08Z
ordinal: 26
note: New key, a check finding. German "{detail} trägt ein attachments-Verzeichnis, und Dinah führt an {detail} keine Anhänge, sodass nichts die Dateien darin erreicht" and Hindi "{detail} में एक attachments निर्देशिका है, और Dinah {detail} पर कोई संलग्नक नहीं रखती, इसलिए उसके भीतर की फ़ाइलों तक कुछ नहीं पहुँचता". The literal directory name attachments is left in English in both, because it is the name of a directory on disk rather than a word about one, and {detail} carries a machine token the context declares is never translated. The register follows check.attachment-filename-drift and check.dangling-workstream, the neighbouring findings. Source stamped with msg.Fingerprint of the English.
---
`check.attachments-without-a-mount`: written fresh in German and Hindi against the current English and its context.