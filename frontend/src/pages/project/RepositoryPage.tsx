import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { Alert, Breadcrumb, Button, Empty, Input, List, Typography } from 'antd'
import { FileOutlined, FolderOutlined, HomeOutlined } from '@ant-design/icons'
import * as projectsApi from '@/api/projects'
import { pullRepository, pushRepository } from '@/api/projects'
import type { FileEntry } from '@/api/projects'

export default function RepositoryPage() {
  const { id = '' } = useParams()
  const [path, setPath] = useState('')
  const [files, setFiles] = useState<FileEntry[]>([])
  const [content, setContent] = useState('')
  const [selected, setSelected] = useState('')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [info, setInfo] = useState('')

  async function loadDir(p = '') {
    setPath(p)
    const res = await projectsApi.listFiles(id, p)
    setFiles(res.files)
    setContent('')
    setSelected('')
  }

  async function openFile(file: FileEntry) {
    if (file.is_dir) {
      await loadDir(file.path)
      return
    }
    setSelected(file.path)
    const res = await projectsApi.readFile(id, file.path)
    setContent(res.content)
  }

  async function pull() {
    setError('')
    setInfo('')
    try {
      await pullRepository(id)
      setInfo('拉取成功')
      await loadDir(path)
    } catch (e) {
      setError(e instanceof Error ? e.message : '拉取失败')
    }
  }

  async function push() {
    setError('')
    setInfo('')
    try {
      await pushRepository(id, message)
      setInfo('推送成功')
    } catch (e) {
      setError(e instanceof Error ? e.message : '推送失败')
    }
  }

  useEffect(() => { loadDir() }, [id])

  const pathParts = path ? path.split('/').filter(Boolean) : []

  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: 12, marginBottom: 12 }}>
        <h2>仓库</h2>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
          <Input value={message} onChange={(e) => setMessage(e.target.value)} placeholder="提交说明" style={{ minWidth: 200, width: 320 }} />
          <Button onClick={pull}>拉取</Button>
          <Button type="primary" onClick={push}>推送</Button>
        </div>
      </div>
      {error && <Alert type="error" message={error} style={{ marginBottom: 12 }} />}
      {info && <Alert type="success" message={info} style={{ marginBottom: 12 }} />}
      <div style={{ display: 'grid', gridTemplateColumns: '280px 1fr', gap: 16 }}>
        <aside className="panel">
          <Breadcrumb
            style={{ marginBottom: 8 }}
            items={[
              {
                title: (
                  <Button type="link" size="small" icon={<HomeOutlined />} onClick={() => loadDir('')} style={{ padding: 0 }}>
                    根目录
                  </Button>
                ),
              },
              ...pathParts.map((part, index) => ({
                title: (
                  <Button
                    type="link"
                    size="small"
                    style={{ padding: 0 }}
                    onClick={() => loadDir(pathParts.slice(0, index + 1).join('/'))}
                  >
                    {part}
                  </Button>
                ),
              })),
            ]}
          />
          <List
            dataSource={files}
            locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="空目录" /> }}
            renderItem={(f) => (
              <List.Item style={{ padding: '4px 0' }}>
                <Button
                  type="link"
                  icon={f.is_dir ? <FolderOutlined /> : <FileOutlined />}
                  onClick={() => openFile(f)}
                  style={{ padding: 0, height: 'auto' }}
                >
                  {f.name}
                </Button>
              </List.Item>
            )}
          />
        </aside>
        <section className="panel">
          {selected && <Typography.Title level={5}>{selected}</Typography.Title>}
          {content ? (
            <Typography.Paragraph>
              <pre style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word', fontSize: 12, margin: 0 }}>{content}</pre>
            </Typography.Paragraph>
          ) : (
            <Empty description="选择文件查看内容" />
          )}
        </section>
      </div>
    </div>
  )
}
