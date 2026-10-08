import { describe, expect, it } from "vitest";
import { moveItem, orderByIds } from "./order";

describe("ordering", () => {
  it("moves an item up or down", () => {
    expect(moveItem(["a", "b", "c"], 2, -1)).toEqual(["a", "c", "b"]);
    expect(moveItem(["a", "b", "c"], 0, 1)).toEqual(["b", "a", "c"]);
  });

  it("returns an unchanged copy past either end", () => {
    const items = ["a", "b"];
    const moved = moveItem(items, 0, -1);
    expect(moved).toEqual(items);
    expect(moved).not.toBe(items);
    expect(moveItem(items, 1, 1)).toEqual(items);
  });

  it("orders by ids and keeps unlisted items last", () => {
    const items = [{ id: "a" }, { id: "b" }, { id: "c" }];
    expect(orderByIds(items, ["c", "a"]).map((x) => x.id)).toEqual(["c", "a", "b"]);
  });
});
