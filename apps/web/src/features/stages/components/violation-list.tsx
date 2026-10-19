import { summarizeViolations } from "../model/lesson-form";

/** Image sources and link targets the server would refuse, the first few listed. */
export function ViolationList({ violations }: { violations: string[] }) {
  const { shown, more } = summarizeViolations(violations);
  return (
    <>
      Gỡ các ảnh/liên kết không hợp lệ trước khi lưu:
      <ul className="mt-1 list-disc pl-5">
        {shown.map((url, i) => (
          <li key={`${String(i)}:${url}`} className="break-all">
            {url}
          </li>
        ))}
      </ul>
      {more > 0 ? <p>và {more} mục khác</p> : null}
    </>
  );
}
