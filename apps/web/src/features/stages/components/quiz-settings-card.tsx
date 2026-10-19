import { Card, CardBody, CardHeader, CardTitle } from "@/shared/ui/card";
import { QUIZ_KIND_LABEL } from "../model/lesson-form";
import type { Quiz } from "../model/schemas";
import { Facts, type Fact } from "./facts";

/** What a learner and a teacher get from a quiz, in the prototype's words. */
function quizFacts(quiz: Quiz): Fact[] {
  const assessment = quiz.mode === "assessment";
  const facts: Fact[] = [
    {
      label: "Loại quiz",
      value: `${QUIZ_KIND_LABEL[quiz.mode]} (${assessment ? "tính vào tiến độ" : "luyện tập"})`,
    },
  ];
  // A practice quiz has no pass mark: it counts as done once a try is submitted.
  if (quiz.passPercent !== null) facts.push({ label: "Ngưỡng đạt", value: `${String(quiz.passPercent)}%` });
  facts.push(
    {
      label: "Số lượt",
      value: quiz.maxAttempts === null ? "Không giới hạn" : `${String(quiz.maxAttempts)} lượt`,
    },
    {
      label: "Thời gian",
      value: quiz.timeLimitMinutes === null ? "Không giới hạn" : `${String(quiz.timeLimitMinutes)} phút`,
    },
    { label: "Xáo trộn", value: quiz.shuffle ? "Câu hỏi và phương án" : "Giữ thứ tự soạn" },
    { label: "Cách chấm", value: "Đúng toàn bộ phương án mới được điểm câu" },
    {
      label: "Giảng viên",
      value: assessment ? "Xác nhận từng lượt nộp trong hàng chờ chấm" : "Xem và nhận xét khi cần",
    },
  );
  return facts;
}

/** "Thiết lập" card of a quiz: its settings, then what the learner sees after submitting. */
export function QuizSettingsCard({ quiz }: { quiz: Quiz }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Thiết lập</CardTitle>
      </CardHeader>
      <CardBody className="grid gap-3">
        <Facts items={quizFacts(quiz)} />
        <p className="text-xs text-ink-3">
          {quiz.mode === "assessment"
            ? "Sau khi nộp, học viên chỉ thấy điểm và kết quả Đạt hoặc Chưa đạt, không thấy đáp án. Lượt tốt nhất được tính."
            : "Sau khi nộp, học viên thấy đáp án đúng và giải thích từng câu."}
        </p>
      </CardBody>
    </Card>
  );
}
