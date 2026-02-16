import { useEffect, useMemo } from 'react'
import ReactFlow, { Node, Edge, Controls, Background, useNodesState, useEdgesState, MarkerType } from 'reactflow'
import 'reactflow/dist/style.css'
import { Task } from '../lib/api'

interface SubtaskDAGVisualizationProps { nodes: Task[]; edges: { dependent_id: string; dependency_id: string }[] }

const getNodeColor = (status: string): string => {
  switch (status) {
    case 'COMPLETED': return '#10b981'
    case 'RUNNING': return '#f59e0b'
    case 'FAILED': return '#f43f5e'
    case 'READY': return '#06b6d4'
    case 'DECOMPOSED': return '#8b5cf6'
    case 'CANCELLED': return '#475569'
    case 'PENDING': case 'ARCHIVED': default: return '#334155'
  }
}

const computeLayout = (tasks: Task[], dependencies: { dependent_id: string; dependency_id: string }[]): { nodes: Node[]; edges: Edge[] } => {
  const outgoing = new Map<string, string[]>(); const incoming = new Map<string, string[]>()
  tasks.forEach((task) => { outgoing.set(task.id, []); incoming.set(task.id, []) })
  dependencies.forEach((dep) => { outgoing.get(dep.dependency_id)?.push(dep.dependent_id); incoming.get(dep.dependent_id)?.push(dep.dependency_id) })
  const layers: string[][] = []; const layerMap = new Map<string, number>(); const queue: string[] = []
  tasks.forEach((task) => { if ((incoming.get(task.id)?.length ?? 0) === 0) { queue.push(task.id); layerMap.set(task.id, 0) } })
  while (queue.length > 0) {
    const nodeId = queue.shift()!; const layer = layerMap.get(nodeId)!
    if (!layers[layer]) layers[layer] = []; layers[layer].push(nodeId)
    outgoing.get(nodeId)?.forEach((child) => {
      const childIncoming = incoming.get(child)!
      if (childIncoming.every((parent) => layerMap.has(parent))) { layerMap.set(child, Math.max(...childIncoming.map((p) => layerMap.get(p)!)) + 1); queue.push(child) }
    })
  }
  tasks.forEach((task) => { if (!layerMap.has(task.id)) { const lastLayer = layers.length; if (!layers[lastLayer]) layers[lastLayer] = []; layers[lastLayer].push(task.id); layerMap.set(task.id, lastLayer) } })
  const taskMap = new Map(tasks.map((t) => [t.id, t]))
  const nodes: Node[] = []; const nodeSpacingX = 280; const nodeSpacingY = 100
  layers.forEach((layer, layerIndex) => {
    layer.forEach((taskId, indexInLayer) => {
      const task = taskMap.get(taskId)!
      nodes.push({ id: task.id, type: 'default', data: { label: (<div className="text-xs text-center"><div className="font-semibold text-white truncate max-w-[180px]">{task.title}</div><div className="text-[10px] mt-1 text-white/70">{task.status}</div></div>) }, position: { x: indexInLayer * nodeSpacingX + 100, y: layerIndex * nodeSpacingY + 50 }, style: { background: getNodeColor(task.status), border: '1px solid rgba(0, 0, 0, 0.08)', borderRadius: '10px', padding: '12px', minWidth: '200px', color: 'white', boxShadow: '0 2px 8px rgba(0,0,0,0.1)' } })
    })
  })
  const edges: Edge[] = dependencies.map((dep) => ({ id: `${dep.dependency_id}-${dep.dependent_id}`, source: dep.dependency_id, target: dep.dependent_id, type: 'smoothstep', animated: taskMap.get(dep.dependent_id)?.status === 'RUNNING', style: { stroke: 'rgba(0,0,0,0.15)', strokeWidth: 2 }, markerEnd: { type: MarkerType.ArrowClosed, color: 'rgba(0,0,0,0.15)' } }))
  return { nodes, edges }
}

export default function SubtaskDAGVisualization({ nodes: tasks, edges: dependencies }: SubtaskDAGVisualizationProps) {
  const layout = useMemo(() => computeLayout(tasks, dependencies), [tasks, dependencies])
  const [nodes, setNodes, onNodesChange] = useNodesState(layout.nodes)
  const [edges, setEdges, onEdgesChange] = useEdgesState(layout.edges)
  useEffect(() => { setNodes(layout.nodes); setEdges(layout.edges) }, [layout, setNodes, setEdges])

  return (
    <div style={{ width: '100%', height: '600px' }} className="bg-surface-deep rounded-xl border border-line">
      <ReactFlow nodes={nodes} edges={edges} onNodesChange={onNodesChange} onEdgesChange={onEdgesChange} fitView fitViewOptions={{ padding: 0.2, maxZoom: 1 }} minZoom={0.1} maxZoom={1.5} attributionPosition="bottom-left">
        <Controls />
        <Background color="rgba(0,0,0,0.04)" gap={16} />
      </ReactFlow>
    </div>
  )
}
