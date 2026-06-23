import { Select, Typography } from "antd";
import type { SelectProps } from "antd";

export type DocFileOption = {
  value: string;
  label: string;
  path: string;
  content?: string;
};

type DocFileSelectProps = {
  options: DocFileOption[];
  value?: string;
  onChange?: (value: string) => void;
  placeholder?: string;
  allowClear?: boolean;
  disabled?: boolean;
};

export default function DocFileSelect({
  options,
  value,
  onChange,
  placeholder = "选择文档",
  allowClear = true,
  disabled = false,
}: DocFileSelectProps) {
  const selectOptions: SelectProps["options"] = options.map((o) => ({
    value: o.value,
    label: o.label,
    path: o.path,
  }));
  return (
    <Select
      allowClear={allowClear}
      showSearch
      disabled={disabled}
      placeholder={placeholder}
      value={value || undefined}
      onChange={(v) => onChange?.(v ?? "")}
      options={selectOptions}
      optionFilterProp="label"
      filterOption={(input, option) => {
        const q = input.toLowerCase();
        const label = String(option?.label ?? "").toLowerCase();
        const path = String(
          (option as { path?: string })?.path ?? "",
        ).toLowerCase();
        return label.includes(q) || path.includes(q);
      }}
      optionRender={(option) => (
        <div>
          <div>{option.label}</div>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {(option.data as { path?: string })?.path ?? option.value}
          </Typography.Text>
        </div>
      )}
      data-testid="doc-file-select"
    />
  );
}
