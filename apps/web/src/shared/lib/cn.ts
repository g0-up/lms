import { createCn } from "cn/config";

/**
 * Class merger aware of the GoUp theme scale (app.css), so custom tokens such as
 * `shadow-card`, `rounded-pill` or `animate-pop` resolve conflicts like built-in ones.
 */
export const cn = createCn({
  extend: {
    theme: {
      shadow: ["card", "card-strong", "float", "control"],
      radius: ["pill"],
      animate: ["pop", "slide", "rise"],
      ease: ["brand"],
    },
  },
});
