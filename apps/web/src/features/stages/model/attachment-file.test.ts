import { describe, expect, it } from "vitest";
import {
  DOCUMENT_ACCEPT,
  DOCUMENT_SIZE_MESSAGE,
  DOCUMENT_TYPE_MESSAGE,
  MAX_DOCUMENT_BYTES,
  documentFileError,
} from "./attachment-file";

describe("documentFileError", () => {
  it("accepts every allowed extension whatever its case", () => {
    for (const name of [
      "a.zip",
      "de-bai.PDF",
      "README.md",
      "ghi-chu.txt",
      "schema.sql",
      "so-do.png",
      "x.JPG",
      "y.jpeg",
    ]) {
      expect(documentFileError({ name, size: 10 })).toBeNull();
    }
  });

  it("refuses other extensions and names without one", () => {
    expect(documentFileError({ name: "diem.csv", size: 10 })).toBe(DOCUMENT_TYPE_MESSAGE);
    expect(documentFileError({ name: "trang.html", size: 10 })).toBe(DOCUMENT_TYPE_MESSAGE);
    expect(documentFileError({ name: "Makefile", size: 10 })).toBe(DOCUMENT_TYPE_MESSAGE);
    expect(documentFileError({ name: ".pdf", size: 10 })).toBe(DOCUMENT_TYPE_MESSAGE);
  });

  it("refuses a file over 20 MB and accepts one of exactly 20 MB", () => {
    expect(documentFileError({ name: "a.zip", size: MAX_DOCUMENT_BYTES })).toBeNull();
    expect(documentFileError({ name: "a.zip", size: MAX_DOCUMENT_BYTES + 1 })).toBe(DOCUMENT_SIZE_MESSAGE);
  });

  it("lists the extensions for the file picker", () => {
    expect(DOCUMENT_ACCEPT).toBe(".zip,.pdf,.md,.txt,.sql,.png,.jpg,.jpeg");
  });
});
