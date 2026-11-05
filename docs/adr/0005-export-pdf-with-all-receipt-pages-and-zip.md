---
status: accepted
date: 2026-10-19
---
# Export one PDF with every receipt page, plus an evidence ZIP

A submitted claim yields one Typst-rendered PDF containing the summary, calculation details and all numbered receipt pages, plus a ZIP with the PDF, originals, JSON/CSV and a SHA-256 manifest. This supersedes the earlier reduced scope in which the PDF only referred to evidence in the ZIP, because the owner wants a single readable document for the employer. Typst was kept because it embeds receipt images and PDF pages and is a runtime binary in the image. PDF/A remains an extension.
