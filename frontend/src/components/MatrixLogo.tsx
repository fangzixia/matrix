import { Link } from 'react-router-dom'
import { Space, Typography } from 'antd'

interface MatrixLogoProps {
  showText?: boolean
  size?: number
}

/** 品牌 Logo（Ant Design 无 Logo 组件，基于 Typography 二次封装） */
export function MatrixLogo({ showText = true, size = 24 }: MatrixLogoProps) {
  return (
    <Space size={8} className="matrix-logo">
      <svg className="matrix-logo__mark" viewBox="0 0 36 36" width={size} height={size} aria-hidden="true">
        <path fill="#e24329" d="M18 0C8.06 0 0 8.06 0 18s8.06 18 18 18 18-8.06 18-18S27.94 0 18 0z" />
        <path fill="#fc6d26" d="M18 4.5L6 18h4.5l7.5-9 7.5 9H30L18 4.5z" />
        <path fill="#fca326" d="M18 36c4.97 0 9.45-2.01 12.7-5.25L18 13.5 5.3 30.75A17.93 17.93 0 0 0 18 36z" />
      </svg>
      {showText && (
        <Typography.Text strong className="matrix-logo__text" style={{ fontSize: 16, color: '#e24329' }}>
          Matrix
        </Typography.Text>
      )}
    </Space>
  )
}

export function MatrixLogoLink({ to = '/projects', ...props }: MatrixLogoProps & { to?: string }) {
  return (
    <Link to={to} className="top-bar__logo" title="工作台">
      <MatrixLogo {...props} />
    </Link>
  )
}
