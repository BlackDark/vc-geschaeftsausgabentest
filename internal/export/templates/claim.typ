// Claim summary. Data comes from data.json (the frozen snapshot plus
// display labels); user content is only ever passed as data.
#let d = json("data.json")
#let s = d.snapshot
#let lang = d.locale
#let L = d.labels
#let accent = rgb("#1e40af")
#let muted = rgb("#64748b")
#let line-color = rgb("#e2e8f0")
#let soft = rgb("#f1f5f9")

#set document(title: L.title + " " + s.claim.number, author: s.user.name)
#set text(font: "Inter", size: 9pt, lang: lang, fill: rgb("#0f172a"))
#set par(leading: 0.55em)
#set page(
  paper: "a4",
  margin: (top: 22mm, bottom: 20mm, x: 17mm),
  header: context {
    if counter(page).get().first() > 1 {
      set text(size: 7.5pt, fill: muted)
      grid(columns: (1fr, auto), s.user.name + " · " + s.employer.name, L.claim + " " + s.claim.number + " · " + L.version + " " + str(s.claim.version))
      v(-4pt)
      line(length: 100%, stroke: 0.5pt + line-color)
    }
  },
  footer: context {
    set text(size: 7.5pt, fill: muted)
    grid(columns: (1fr, auto),
      L.generated + " " + d.generatedAt,
      L.page + " " + str(counter(page).get().first()) + " / " + str(counter(page).final().first()))
  },
)

#let money(c) = {
  let neg = c < 0
  let v = calc.abs(c)
  let euros = str(calc.quo(v, 100))
  let cents = calc.rem(v, 100)
  let parts = ()
  while euros.len() > 3 {
    parts.insert(0, euros.slice(euros.len() - 3))
    euros = euros.slice(0, euros.len() - 3)
  }
  parts.insert(0, euros)
  let (sep, dec) = if lang == "de" { (".", ",") } else { (",", ".") }
  let num = parts.join(sep) + dec + (if cents < 10 { "0" } else { "" }) + str(cents)
  let sign = if neg { "−" } else { "" }
  if lang == "de" { sign + num + " €" } else { sign + "€" + num }
}
#let date(x) = {
  if x == none or x == "" { return "" }
  let p = x.slice(0, 10).split("-")
  if lang == "de" { p.at(2) + "." + p.at(1) + "." + p.at(0) } else { x.slice(0, 10) }
}
#let datetime-local(x) = date(x) + " " + x.slice(11, 16)
#let lab(dict, key) = dict.at(key, default: key)
#let place(loc) = {
  let name = d.countries.at(loc.country, default: loc.country)
  if loc.at("city", default: "") != "" { loc.city + " (" + name + ")" } else { name }
}
#let num-cell(x) = align(right, x)
#let km(m) = {
  let v = calc.round(m / 1000, digits: 1)
  let t = str(v)
  if lang == "de" { t.replace(".", ",") + " km" } else { t + " km" }
}

#let section(title) = {
  v(10pt)
  block(below: 6pt, text(size: 12pt, weight: "semibold", fill: accent, title))
}
#let subsection(title) = block(above: 9pt, below: 5pt, text(size: 9.5pt, weight: "semibold", title))

#let data-table(columns: (), align-right: (), header: (), rows: (), footer: none) = {
  set text(size: 8.3pt)
  let cols = columns.len()
  table(
    columns: columns,
    stroke: (x, y) => (bottom: if y == 0 { 0.8pt + accent } else { 0.4pt + line-color }),
    fill: (x, y) => if y > 0 and calc.even(y) { soft } else { none },
    inset: (x: 5pt, y: 4.5pt),
    align: (x, y) => if align-right.contains(x) { right + horizon } else { left + horizon },
    table.header(..header.map(h => text(weight: "semibold", fill: muted, size: 7.6pt, upper(h)))),
    ..rows.flatten(),
    ..if footer != none { footer.map(c => text(weight: "semibold", c)) } else { () },
  )
}

#let kv(pairs) = grid(
  columns: (auto, 1fr),
  column-gutter: 10pt,
  row-gutter: 5pt,
  ..pairs.filter(p => p.at(1) != none and p.at(1) != "").map(p => (text(fill: muted, p.at(0)), p.at(1))).flatten(),
)

// ── Title ──────────────────────────────────────────────────────────────
#grid(
  columns: (1fr, auto),
  align: (left + bottom, right + bottom),
  [
    #text(size: 8pt, fill: muted, upper(L.kicker))
    #v(-4pt)
    #text(size: 20pt, weight: "bold", L.title)
  ],
  box(fill: soft, inset: (x: 10pt, y: 7pt), radius: 4pt)[
    #set text(size: 8pt)
    #grid(columns: (auto, auto), column-gutter: 8pt, row-gutter: 4pt, align: (left, right),
      text(fill: muted, L.number), text(weight: "semibold", s.claim.number),
      text(fill: muted, L.version), str(s.claim.version),
      text(fill: muted, L.period), date(s.claim.periodStart) + " – " + date(s.claim.periodEnd),
      text(fill: muted, L.submitted), date(s.claim.submittedAt),
    )
  ],
)
#v(8pt)
#line(length: 100%, stroke: 1.2pt + accent)
#v(6pt)

#grid(
  columns: (1fr, 1fr),
  column-gutter: 18pt,
  [
    #text(size: 7.6pt, weight: "semibold", fill: muted, upper(L.employee))
    #v(-2pt)
    #text(size: 10.5pt, weight: "semibold", s.user.name) \
    #if s.user.address != "" [#s.user.address.replace("\n", ", ") \ ]
    #kv(((L.employeeNumber, s.user.employeeNumber), (L.iban, s.user.iban), (L.email, s.user.email)))
  ],
  [
    #text(size: 7.6pt, weight: "semibold", fill: muted, upper(L.employer))
    #v(-2pt)
    #text(size: 10.5pt, weight: "semibold", s.employer.name) \
    #s.employer.address.replace("\n", ", ")
  ],
)
#if s.claim.title != "" {
  v(6pt)
  text(fill: muted, L.claimTitle + ": ") + s.claim.title
}
#if s.claim.at("previousUnlockReason", default: "") != "" {
  v(4pt)
  block(fill: rgb("#fef9c3"), inset: 7pt, radius: 3pt, width: 100%, text(size: 8pt)[*#L.correction:* #s.claim.previousUnlockReason])
}

// ── Summary ────────────────────────────────────────────────────────────
#section(L.summary)
#let t = s.totals
#block(stroke: 0.8pt + line-color, radius: 5pt, inset: 0pt, clip: true, width: 100%)[
  #set text(size: 9.5pt)
  #table(
    columns: (1fr, auto),
    stroke: (x, y) => (bottom: 0.4pt + line-color),
    inset: (x: 10pt, y: 6pt),
    align: (left, right),
    L.travelTotal + " (" + str(s.trips.len()) + ")", money(t.travel),
    L.outlaysTotal + " (" + str(s.outlays.len()) + ")", money(t.outlays),
    text(weight: "semibold", L.reimbursement), text(weight: "semibold", money(t.reimbursement)),
    L.lessAdvances, if t.advances > 0 { "− " + money(t.advances) } else { money(0) },
  )
  #block(fill: accent, inset: (x: 10pt, y: 9pt), width: 100%, below: 0pt)[
    #set text(fill: white, size: 11pt, weight: "bold")
    #grid(columns: (1fr, auto), if t.repayment { L.repayment } else { L.payout }, money(calc.abs(t.payout)))
  ]
]
#if t.repayment { text(size: 8pt, fill: muted, L.repaymentHint) }

// ── Trips overview ─────────────────────────────────────────────────────
#if s.trips.len() > 0 {
  section(L.trips)
  data-table(
    columns: (auto, auto, 1fr, auto, auto, auto, auto, auto),
    align-right: (3, 4, 5, 6, 7),
    header: ("#", L.when, L.purpose, L.meals, L.lodging, L.transportCol, L.otherCol, L.total),
    rows: s.trips.enumerate().map(((i, tr)) => {
      let tt = tr.result.totals
      let by = tt.byCostType
      let lodging = tt.lodgingAllowance + by.at("lodging", default: 0)
      let transport = tt.mileage + by.at("transport", default: 0)
      let other = tt.total - tt.meals - lodging - transport
      (str(i + 1), date(tr.start) + " – " + date(tr.end),
       [#tr.purpose #if tr.destination != "" [#text(fill: muted, " · " + tr.destination)]],
       money(tt.meals), money(lodging), money(transport), money(other), text(weight: "semibold", money(tt.total)))
    }),
    footer: ("", "", L.sum, "", "", "", "", money(t.travel)),
  )
}

// ── Standalone outlays ─────────────────────────────────────────────────
#if s.outlays.len() > 0 {
  section(L.outlays)
  data-table(
    columns: (auto, 1fr, auto, auto, auto, auto),
    align-right: (4, 5),
    header: (L.date, L.supplierPurpose, L.category, L.receipt, L.gross, L.reimbursed),
    rows: s.outlays.map(e => (
      date(e.date),
      [#e.supplier #if e.description != "" [#text(fill: muted, " · " + e.description)]],
      lab(L.costTypes, e.costType),
      e.receipts.join(", "),
      money(e.result.grossEur),
      text(weight: "semibold", money(e.result.reimbursable)),
    )),
    footer: ("", L.sum, "", "", "", money(t.outlays)),
  )
}

// ── Advances ───────────────────────────────────────────────────────────
#if s.advances.len() > 0 {
  section(L.advances)
  data-table(
    columns: (auto, 1fr, auto),
    align-right: (2,),
    header: (L.date, L.note, L.amount),
    rows: s.advances.map(a => (date(a.date), a.note, money(a.amount))),
    footer: ("", L.sum, money(t.advances)),
  )
}

// ── Trip details ───────────────────────────────────────────────────────
#for (i, tr) in s.trips.enumerate() {
  pagebreak(weak: true)
  let r = tr.result
  block(below: 4pt, text(size: 13pt, weight: "bold", fill: accent, L.trip + " " + str(i + 1) + ": " + tr.purpose))
  grid(columns: (1fr, 1fr), column-gutter: 18pt,
    kv(((L.departure, datetime-local(tr.start) + " (" + tr.startTz + ")"), (L.return, datetime-local(tr.end) + " (" + tr.endTz + ")"))),
    kv(((L.destination, tr.destination), (L.project, tr.project), (L.notes, tr.notes))),
  )
  if tr.stops.len() > 0 {
    subsection(L.itinerary)
    data-table(columns: (auto, 1fr, auto), header: (L.arrival, L.location, L.business),
      rows: tr.stops.map(st => (datetime-local(st.arrival), place(st.location), if st.business { L.yes } else { L.no })))
  }

  subsection(L.mealAllowance)
  data-table(
    columns: (auto, auto, 1fr, auto, 1.2fr, auto),
    align-right: (3, 5),
    header: (L.date, L.dayType, L.rateLocation, L.allowance, L.deductions, L.amount),
    rows: r.days.map(day => {
      let ded = day.deductions.map(x => lab(L.mealsNames, x.meal) + " −" + money(x.amount) + if x.contribution > 0 { " (" + L.contribution + " " + money(x.contribution) + ")" } else { "" }).join(", ")
      let notes = ()
      if day.excluded { notes.push(L.excluded) }
      if day.at("allocatedTo", default: 0) != 0 { notes.push(L.allocatedElsewhere) }
      if day.frozenElsewhere > 0 { notes.push(L.frozenElsewhere + " " + money(day.frozenElsewhere)) }
      let typ = lab(L.dayTypes, day.type)
      if day.type == "one_day" or day.type == "none" {
        typ = typ + " (" + str(calc.quo(day.absenceMinutes, 60)) + ":" + (if calc.rem(day.absenceMinutes, 60) < 10 { "0" } else { "" }) + str(calc.rem(day.absenceMinutes, 60)) + " h)"
      }
      (date(day.date), typ, place(day.location), money(day.base),
       [#ded #if notes.len() > 0 [#text(fill: muted, notes.join(", "))]],
       text(weight: "semibold", money(day.payable)))
    }),
    footer: ("", "", L.sum, "", "", money(r.totals.meals)),
  )

  if r.nights.len() > 0 {
    subsection(L.lodging)
    data-table(columns: (auto, 1fr, 1fr, auto), align-right: (3,),
      header: (L.night, L.arrangement, L.rateLocation, L.allowance),
      rows: r.nights.map(n => (date(n.date), lab(L.nightKinds, n.kind), place(n.location),
        if n.kind == "private_actual" { text(fill: muted, L.viaInvoice) } else { money(n.allowance) })),
      footer: ("", L.sum, "", money(r.totals.lodgingAllowance)))
  }

  if r.journeys.len() > 0 {
    subsection(L.mileage)
    data-table(columns: (auto, 1fr, auto, auto, auto), align-right: (2, 3, 4),
      header: (L.date, L.vehicle, L.distance, L.ratePerKm, L.amount),
      rows: r.journeys.map(j => (date(j.date), lab(L.vehicles, j.vehicle), km(j.distanceMeters), money(j.centsPerKm) + "/km", money(j.amount))),
      footer: ("", L.sum, "", "", money(r.totals.mileage)))
  }

  if tr.expenses.len() > 0 {
    subsection(L.expenses)
    data-table(columns: (auto, 1fr, auto, auto, auto, auto, auto), align-right: (5, 6),
      header: (L.date, L.supplierPurpose, L.costType, L.paidBy, L.receipt, L.gross, L.reimbursed),
      rows: tr.expenses.map(e => {
        let gross = money(e.result.grossEur)
        if e.currency != "EUR" {
          gross = [#gross #text(fill: muted, size: 7pt)[\ #e.currency #str(e.grossMinor / calc.pow(10, e.exponent))]]
        }
        (date(e.date),
         [#e.supplier #if e.description != "" [#text(fill: muted, " · " + e.description)] #if e.result.at("reason", default: "") != "" [#text(fill: muted, size: 7.4pt)[\ #lab(L.reasons, e.result.reason)]]],
         lab(L.costTypes, e.costType), lab(L.payers, e.payer), e.receipts.join(", "), gross, text(weight: "semibold", money(e.result.reimbursable)))
      }),
      footer: ("", L.sum, "", "", "", "", money(r.totals.expenses)))
    let cp = r.totals.companyPaid
    if cp > 0 { text(size: 7.8pt, fill: muted, L.companyPaidNote + " " + money(cp)) }
  }

  v(8pt)
  block(fill: soft, inset: (x: 10pt, y: 7pt), radius: 4pt, width: 100%,
    grid(columns: (1fr, auto), text(weight: "semibold", L.tripTotal), text(weight: "bold", money(r.totals.total))))
}

// ── Notes, sources, confirmation ───────────────────────────────────────
#pagebreak(weak: true)
#if s.issues.len() > 0 {
  section(L.notesTitle)
  text(size: 8pt, fill: muted, L.notesIntro)
  v(2pt)
  for is in s.issues {
    let sev = lab(L.severities, is.severity)
    block(below: 4pt, text(size: 8.3pt)[• *#sev* #lab(L.issueCodes, is.code) #if is.at("date", default: "") != "" [(#date(is.date))] #text(fill: muted, "[" + is.ruleId + "]")])
  }
}

#section(L.sources)
#text(size: 8pt, fill: muted, L.sourcesIntro)
#v(2pt)
#for src in d.sources [
  #block(below: 3pt, text(size: 8pt)[• #src])
]
#v(3pt)
#text(size: 7.6pt, fill: muted, L.disclaimer)

#section(L.confirmation)
#text(size: 8.5pt, L.confirmationText)
#v(26pt)
#grid(columns: (1fr, 1fr), column-gutter: 30pt,
  [#line(length: 100%, stroke: 0.6pt) #text(size: 7.6pt, fill: muted, L.signEmployee)],
  [#line(length: 100%, stroke: 0.6pt) #text(size: 7.6pt, fill: muted, L.signEmployer)],
)

// ── Receipts ───────────────────────────────────────────────────────────
#if s.receipts.len() > 0 {
  pagebreak()
  section(L.receiptIndex)
  data-table(columns: (auto, 1fr, auto, auto), align-right: (2,),
    header: (L.receipt, L.file, L.pages, "SHA-256"),
    rows: s.receipts.map(rc => (text(weight: "semibold", rc.number), rc.filename, str(rc.pages), text(font: "DejaVu Sans Mono", size: 6.5pt, rc.sha256.slice(0, 16) + "…"))))
  text(size: 7.6pt, fill: muted, L.receiptIndexNote)

  for rc in s.receipts {
    if rc.kind == "self_issued" {
      pagebreak()
      let si = rc.selfIssued
      block(below: 8pt, text(size: 13pt, weight: "bold", fill: accent, L.selfIssued + " " + rc.number))
      kv(((L.date, date(si.date)), (L.amount, if si.currency == "EUR" { money(si.amountMinor) } else { si.currency + " " + str(si.amountMinor / 100) }), (L.payee, si.payee), (L.purpose, si.purpose), (L.issuer, si.issuer), (L.missingReason, si.reason)))
      v(30pt)
      line(length: 50%, stroke: 0.6pt)
      text(size: 7.6pt, fill: muted, L.signEmployee)
    } else {
      for p in range(1, rc.pages + 1) {
        pagebreak()
        block(below: 6pt, grid(columns: (1fr, auto),
          text(weight: "semibold", size: 10pt, fill: accent, L.receipt + " " + rc.number),
          text(size: 8pt, fill: muted, rc.filename + " · " + L.page + " " + str(p) + "/" + str(rc.pages))))
        align(center + horizon, block(height: 92%, width: 100%, align(center + horizon, box(stroke: 0.5pt + line-color, inset: 4pt,
          if rc.mime == "application/pdf" {
            image(rc.zipPath, page: p, fit: "contain", height: 100% - 8pt)
          } else {
            image(rc.zipPath, fit: "contain", height: 100% - 8pt)
          }))))
      }
    }
  }
}
