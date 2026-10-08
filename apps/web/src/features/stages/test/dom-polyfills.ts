/**
 * Browser APIs radix primitives use that jsdom lacks (Checkbox measures itself, Select captures the
 * pointer and scrolls its options), plus the range geometry Lexical reads to place the selection.
 * Test-only: imported by the render helpers before any render.
 */
if (typeof globalThis.ResizeObserver === "undefined") {
  globalThis.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  };
}

const proto = Element.prototype as Partial<Element>;
proto.hasPointerCapture ??= () => false;
proto.releasePointerCapture ??= () => undefined;
proto.scrollIntoView ??= () => undefined;

const range = Range.prototype as Partial<Range>;
range.getBoundingClientRect ??= () => new DOMRect();
range.getClientRects ??= () => Object.assign([], { item: () => null });

export {};
