import { deflateSync } from "node:zlib";
import { expect, type Page } from "@playwright/test";

/** A small synthetic receipt PNG; n varies the bytes. */
export function receiptPng(n: number): Buffer {
  const w = 60;
  const h = 90;
  const raw = Buffer.alloc((w * 3 + 1) * h, 255);
  for (let y = 0; y < h; y++) raw[y * (w * 3 + 1)] = 0; // filter type "none" per row
  for (let y = 10; y < h - 10; y += 8) {
    const len = 20 + ((n * 7 + y) % 30);
    for (let x = 6; x < 6 + len; x++)
      raw.fill(40, y * (w * 3 + 1) + 1 + x * 3, y * (w * 3 + 1) + 1 + x * 3 + 3);
  }
  const chunk = (type: string, data: Buffer) => {
    const len = Buffer.alloc(4);
    len.writeUInt32BE(data.length);
    const td = Buffer.concat([Buffer.from(type), data]);
    const crc = Buffer.alloc(4);
    crc.writeUInt32BE(crc32(td));
    return Buffer.concat([len, td, crc]);
  };
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(w, 0);
  ihdr.writeUInt32BE(h, 4);
  ihdr[8] = 8;
  ihdr[9] = 2;
  return Buffer.concat([
    Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]),
    chunk("IHDR", ihdr),
    chunk("IDAT", deflateSync(raw)),
    chunk("IEND", Buffer.alloc(0)),
  ]);
}

function crc32(buf: Buffer): number {
  let c = ~0;
  for (const b of buf) {
    c ^= b;
    for (let k = 0; k < 8; k++) c = (c >>> 1) ^ (0xedb88320 & -(c & 1));
  }
  return ~c >>> 0;
}

export async function shot(page: Page, name: string) {
  if (process.env.E2E_SCREENSHOTS)
    await page.screenshot({ path: `${process.env.E2E_SCREENSHOTS}/${name}.png`, fullPage: true });
}

/** Creates the first user, or logs in when an earlier spec already did. */
export async function login(page: Page) {
  const setup = await page.request.post("/api/setup", {
    data: { token: "e2e-setup-token", username: "user-a", name: "User A", password: "correct horse battery" },
  });
  if (!setup.ok()) {
    const res = await page.request.post("/api/login", {
      data: { username: "user-a", password: "correct horse battery" },
    });
    expect(res.ok()).toBeTruthy();
  }
}

export async function noHorizontalScroll(page: Page) {
  const overflow = await page.evaluate(
    () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
  );
  expect(overflow, `horizontal overflow on ${page.url()}`).toBeLessThanOrEqual(0);
}

/** Logs in and makes sure an employer exists, so a spec passes when run alone. */
export async function loginWithEmployer(page: Page) {
  await login(page);
  const current = await page.request.get("/api/employer");
  expect(current.ok()).toBeTruthy();
  if ((await current.json()).employer) return;
  const saved = await page.request.put("/api/employer", {
    data: { name: "Example Company", address: "Musterstraße 1\n10115 Berlin" },
  });
  expect(saved.ok()).toBeTruthy();
}
