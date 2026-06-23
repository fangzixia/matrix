import XMarkdown from "@ant-design/x-markdown";

type MarkdownViewProps = {
  content: string;
  className?: string;
  streaming?: boolean;
};

export default function MarkdownView({
  content,
  className,
  streaming = false,
}: MarkdownViewProps) {
  return (
    <XMarkdown
      content={content}
      className={className}
      streaming={
        streaming
          ? { hasNextChunk: true, enableAnimation: false }
          : { hasNextChunk: false }
      }
    />
  );
}
