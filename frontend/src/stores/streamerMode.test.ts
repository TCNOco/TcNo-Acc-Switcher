import { describe, expect, it } from "vitest";
import { censorName, shortenEmails } from "./streamerMode";

describe("shortenEmails", () => {
  it("keeps the local part of an address and drops the domain", () => {
    expect(shortenEmails("someone@example.com")).toBe("someone");
    expect(shortenEmails("first.last+tag@mail.example.co.uk")).toBe("first.last+tag");
  });

  it("shortens an address embedded in a longer name", () => {
    expect(shortenEmails("Main (alt@gmail.com)")).toBe("Main (alt)");
  });

  it("leaves names that merely contain an @ alone", () => {
    // Display names like this are common and carry nothing identifying on their own.
    expect(shortenEmails("@tcno")).toBe("@tcno");
    expect(shortenEmails("me@localhost")).toBe("me@localhost");
  });
});

describe("censorName", () => {
  it("is a no-op with streamer mode off", () => {
    expect(censorName("someone@example.com", false)).toBe("someone@example.com");
  });
});
