/**
 * Vietnamese labels for the MDXEditor UI. Keys come from the editor's own `t(key, default)` calls
 * (pinned version); a key missing here keeps the English default, so a renamed key degrades to
 * English instead of breaking.
 */
const VI: Record<string, string> = {
  "contentArea.editableMarkdown": "Nội dung bài học",

  "toolbar.undo": "Hoàn tác {{shortcut}}",
  "toolbar.redo": "Làm lại {{shortcut}}",
  "toolbar.blockTypeSelect.placeholder": "Kiểu khối",
  "toolbar.blockTypeSelect.selectBlockTypeTooltip": "Chọn kiểu khối",
  "toolbar.blockTypes.paragraph": "Đoạn văn",
  "toolbar.blockTypes.heading": "Tiêu đề {{level}}",
  "toolbar.blockTypes.quote": "Trích dẫn",
  "toolbar.bold": "Đậm",
  "toolbar.removeBold": "Bỏ đậm",
  "toolbar.italic": "Nghiêng",
  "toolbar.removeItalic": "Bỏ nghiêng",
  "toolbar.strikethrough": "Gạch ngang",
  "toolbar.removeStrikethrough": "Bỏ gạch ngang",
  "toolbar.inlineCode": "Mã trong dòng",
  "toolbar.removeInlineCode": "Bỏ mã trong dòng",
  "toolbar.bulletedList": "Danh sách dấu chấm",
  "toolbar.numberedList": "Danh sách đánh số",
  "toolbar.toggleGroup": "Nhóm định dạng",
  "toolbar.link": "Chèn liên kết",
  "toolbar.image": "Chèn ảnh",
  "toolbar.table": "Chèn bảng",
  "toolbar.thematicBreak": "Chèn đường kẻ ngang",
  "toolbar.codeBlock": "Chèn khối mã",
  "toolbar.richText": "Soạn thảo",
  "toolbar.source": "Mã nguồn",
  "toolbar.diffMode": "So sánh",

  "createLink.url": "Địa chỉ",
  "createLink.urlPlaceholder": "Dán địa chỉ liên kết",
  "createLink.text": "Chữ hiển thị",
  "createLink.textTooltip": "Chữ hiển thị cho liên kết",
  "createLink.title": "Tiêu đề liên kết",
  "createLink.titleTooltip": "Hiện khi rê chuột lên liên kết",
  "createLink.saveTooltip": "Lưu liên kết",
  "createLink.cancelTooltip": "Hủy thay đổi",
  "linkPreview.open": "Mở {{url}} trong cửa sổ mới",
  "linkPreview.edit": "Sửa liên kết",
  "linkPreview.copyToClipboard": "Sao chép",
  "linkPreview.copied": "Đã sao chép",
  "linkPreview.remove": "Bỏ liên kết",
  "dialog.close": "Đóng",
  "dialogControls.save": "Lưu",
  "dialogControls.cancel": "Hủy",

  "imageEditor.editImage": "Sửa ảnh",
  "imageEditor.deleteImage": "Xóa ảnh",

  "table.columnMenu": "Tùy chọn cột",
  "table.rowMenu": "Tùy chọn hàng",
  "table.textAlignment": "Căn chữ",
  "table.alignLeft": "Căn trái",
  "table.alignCenter": "Căn giữa",
  "table.alignRight": "Căn phải",
  "table.insertColumnLeft": "Thêm cột bên trái",
  "table.insertColumnRight": "Thêm cột bên phải",
  "table.insertRowAbove": "Thêm hàng phía trên",
  "table.insertRowBelow": "Thêm hàng phía dưới",
  "table.deleteColumn": "Xóa cột này",
  "table.deleteRow": "Xóa hàng này",
  "table.deleteTable": "Xóa bảng",

  "codeBlock.language": "Ngôn ngữ khối mã",
  "codeBlock.selectLanguage": "Chọn ngôn ngữ khối mã",
  "codeBlock.inlineLanguage": "Ngôn ngữ",
  "codeblock.delete": "Xóa khối mã",
};

/** MDXEditor `translation` prop: Vietnamese when known, the editor's default otherwise, `{{name}}` filled in. */
export function translate(key: string, defaultValue: string, interpolations?: Record<string, unknown>): string {
  const template = VI[key] ?? defaultValue;
  if (!interpolations) return template;
  return template.replace(/\{\{(\w+)\}\}/g, (match, name: string) =>
    name in interpolations ? String(interpolations[name]) : match,
  );
}
