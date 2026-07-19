import { Background, Controls, MarkerType, MiniMap, ReactFlow, type Edge, type Node } from "@xyflow/react";
import type { LineageGraph as LineageGraphType } from "../api/types";

const nodeColors: Record<string, string> = {
  mission: "#167c67",
  agent: "#2878b5",
  projection: "#8b5e20",
  lease: "#6b55a3",
  expansion: "#b84a3a",
  approval: "#925f13",
};

export function LineageGraph({ graph }: { graph: LineageGraphType }) {
  const nodes: Node[] = graph.nodes.map((node, index) => ({
    id: node.id,
    data: { label: node.label },
    position: { x: (index % 3) * 250, y: Math.floor(index / 3) * 135 },
    className: "lineage-node",
    style: { borderColor: nodeColors[node.type] ?? "#73808b" },
  }));
  const edges: Edge[] = graph.edges.map((edge, index) => ({
    id: `${edge.from}-${edge.to}-${index}`,
    source: edge.from,
    target: edge.to,
    label: edge.type.replaceAll("_", " "),
    markerEnd: { type: MarkerType.ArrowClosed, color: "#70808c" },
    style: { stroke: "#70808c" },
  }));
  return (
    <div className="lineage-wrap">
      <ReactFlow nodes={nodes} edges={edges} fitView minZoom={0.35} maxZoom={1.6} nodesDraggable={false} nodesConnectable={false}>
        <Background color="#d9e0e5" gap={20} />
        <MiniMap pannable zoomable nodeColor={(node) => String(node.style?.borderColor ?? "#73808b")} />
        <Controls showInteractive={false} />
      </ReactFlow>
      <details className="lineage-fallback">
        <summary>Accessible lineage list</summary>
        <ul>{graph.nodes.map((node) => <li key={node.id}><strong>{node.label}</strong> <span>{node.type}</span></li>)}</ul>
      </details>
    </div>
  );
}
