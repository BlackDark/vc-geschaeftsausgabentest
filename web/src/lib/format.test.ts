import { describe, expect, it } from "vitest";
import { datesBetween, parseAmount, quarterOf, shiftPeriod } from "./format";

describe("parseAmount", () => {
  it("parses German and English decimals", () => {
    expect(parseAmount("12,50")).toBe(1250);
    expect(parseAmount("12.50")).toBe(1250);
    expect(parseAmount("1.234,56")).toBe(123456);
    expect(parseAmount("1,234.56")).toBe(123456);
    expect(parseAmount("178")).toBe(17800);
    expect(parseAmount("178 €")).toBe(17800);
  });
  it("respects the currency exponent", () => {
    expect(parseAmount("1000", 0)).toBe(1000);
    expect(parseAmount("1,5", 0)).toBeNull();
    expect(parseAmount("1.234", 3)).toBe(1234);
  });
  it("rejects garbage", () => {
    expect(parseAmount("abc")).toBeNull();
    expect(parseAmount("")).toBeNull();
    expect(parseAmount("1,234,5")).toBeNull();
  });
});

describe("periods", () => {
  it("finds quarters", () => {
    expect(quarterOf("2026-05-17")).toEqual({ start: "2026-04-01", end: "2026-06-30" });
    expect(shiftPeriod({ start: "2026-04-01", end: "2026-06-30" }, -3)).toEqual({
      start: "2026-01-01",
      end: "2026-03-31",
    });
  });
  it("lists dates across months", () => {
    expect(datesBetween("2026-03-30", "2026-04-01")).toEqual(["2026-03-30", "2026-03-31", "2026-04-01"]);
  });
});
