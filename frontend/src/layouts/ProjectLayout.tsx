import { useEffect } from 'react'
import { Outlet, useParams } from 'react-router-dom'
import { useProjectStore } from '@/stores/project'

export function ProjectLayout() {
  const { id } = useParams()
  const fetchProject = useProjectStore((s) => s.fetchProject)

  useEffect(() => {
    if (id) fetchProject(id)
  }, [id, fetchProject])

  return (
    <div className="project-layout">
      <Outlet />
    </div>
  )
}
