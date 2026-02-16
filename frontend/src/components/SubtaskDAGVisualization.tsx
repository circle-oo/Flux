import { useEffect, useMemo } from 'react'
import ReactFlow, {
  Node,
  Edge,
  Controls,
  Background,
  useNodesState,
  useEdgesState,
  MarkerType,
} from 'reactflow'
import 'reactflow/dist/style.css'
import { Task } from '../lib/api'

interface SubtaskDAGVisualizationProps {
  nodes: Task[]
  edges: { dependent_id: string; dependency_id: string }[]
}

// Map task status to node colors matching badge styles
const getNodeColor = (status: string): string => {
  switch (status) {
    case 'COMPLETED':
      return '#16a34a' // green-600
    case 'RUNNING':
      return '#d97706' // amber-600
    case 'FAILED':
      return '#dc2626' // red-600
    case 'READY':
      return '#2563eb' // blue-600
    case 'DECOMPOSED':
      return '#9333ea' // purple-600
    case 'CANCELLED':
      return '#64748b' // slate-500
    case 'PENDING':
    case 'ARCHIVED':
    default:
      return '#475569' // slate-600
  }
}

// Layered graph layout algorithm (simplified DAG layout)
const computeLayout = (
  tasks: Task[],
  dependencies: { dependent_id: string; dependency_id: string }[]
): { nodes: Node[]; edges: Edge[] } => {
  // Build adjacency lists
  const outgoing = new Map<string, string[]>()
  const incoming = new Map<string, string[]>()
  tasks.forEach((task) => {
    outgoing.set(task.id, [])
    incoming.set(task.id, [])
  })
  dependencies.forEach((dep) => {
    outgoing.get(dep.dependency_id)?.push(dep.dependent_id)
    incoming.get(dep.dependent_id)?.push(dep.dependency_id)
  })

  // Topological sort to determine layers
  const layers: string[][] = []
  const layerMap = new Map<string, number>()
  const queue: string[] = []

  // Start with tasks that have no dependencies
  tasks.forEach((task) => {
    if ((incoming.get(task.id)?.length ?? 0) === 0) {
      queue.push(task.id)
      layerMap.set(task.id, 0)
    }
  })

  while (queue.length > 0) {
    const nodeId = queue.shift()!
    const layer = layerMap.get(nodeId)!

    if (!layers[layer]) {
      layers[layer] = []
    }
    layers[layer].push(nodeId)

    outgoing.get(nodeId)?.forEach((child) => {
      const childIncoming = incoming.get(child)!
      const allParentsProcessed = childIncoming.every((parent) => layerMap.has(parent))
      if (allParentsProcessed) {
        const maxParentLayer = Math.max(...childIncoming.map((p) => layerMap.get(p)!))
        layerMap.set(child, maxParentLayer + 1)
        queue.push(child)
      }
    })
  }

  // Handle cycles or remaining nodes (shouldn't happen in a DAG, but defensive)
  tasks.forEach((task) => {
    if (!layerMap.has(task.id)) {
      const lastLayer = layers.length
      if (!layers[lastLayer]) {
        layers[lastLayer] = []
      }
      layers[lastLayer].push(task.id)
      layerMap.set(task.id, lastLayer)
    }
  })

  // Create task map for quick lookup
  const taskMap = new Map(tasks.map((t) => [t.id, t]))

  // Position nodes
  const nodeSpacingX = 280
  const nodeSpacingY = 100
  const nodes: Node[] = []

  layers.forEach((layer, layerIndex) => {
    const y = layerIndex * nodeSpacingY + 50
    layer.forEach((taskId, indexInLayer) => {
      const task = taskMap.get(taskId)!
      const x = indexInLayer * nodeSpacingX + 100
      nodes.push({
        id: task.id,
        type: 'default',
        data: {
          label: (
            <div className="text-xs text-center">
              <div className="font-semibold text-white truncate max-w-[180px]">{task.title}</div>
              <div className="text-[10px] mt-1 text-slate-300">{task.status}</div>
            </div>
          ),
        },
        position: { x, y },
        style: {
          background: getNodeColor(task.status),
          border: '2px solid rgba(255, 255, 255, 0.2)',
          borderRadius: '8px',
          padding: '12px',
          minWidth: '200px',
          color: 'white',
        },
      })
    })
  })

  // Create edges
  const edges: Edge[] = dependencies.map((dep) => ({
    id: `${dep.dependency_id}-${dep.dependent_id}`,
    source: dep.dependency_id,
    target: dep.dependent_id,
    type: 'smoothstep',
    animated: taskMap.get(dep.dependent_id)?.status === 'RUNNING',
    style: { stroke: '#64748b', strokeWidth: 2 },
    markerEnd: {
      type: MarkerType.ArrowClosed,
      color: '#64748b',
    },
  }))

  return { nodes, edges }
}

export default function SubtaskDAGVisualization({ nodes: tasks, edges: dependencies }: SubtaskDAGVisualizationProps) {
  const layout = useMemo(() => computeLayout(tasks, dependencies), [tasks, dependencies])
  const [nodes, setNodes, onNodesChange] = useNodesState(layout.nodes)
  const [edges, setEdges, onEdgesChange] = useEdgesState(layout.edges)

  // Update nodes and edges when layout changes
  useEffect(() => {
    setNodes(layout.nodes)
    setEdges(layout.edges)
  }, [layout, setNodes, setEdges])

  return (
    <div style={{ width: '100%', height: '600px' }} className="bg-slate-900 rounded-lg border border-slate-700">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        fitView
        fitViewOptions={{ padding: 0.2, maxZoom: 1 }}
        minZoom={0.1}
        maxZoom={1.5}
        attributionPosition="bottom-left"
      >
        <Controls className="bg-slate-800 border border-slate-700" />
        <Background color="#475569" gap={16} />
      </ReactFlow>
    </div>
  )
}
