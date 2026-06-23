import XMarkdown from "@ant-design/x-markdown";

type MarkdownViewProps = {
  content: string;
  className?: string;
};

export default function MarkdownView({
  content,
  className,
}: MarkdownViewProps) {
  return <XMarkdown content={content} className={className} />;
}
