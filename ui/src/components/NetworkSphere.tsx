import { useEffect, useRef } from 'react';

const DATA = {
  "nodes": [[-16.5, -4.6, -229.22], [102.1, -36.5, -202.68], [85.6, 87.9, -194.38], [-130.3, 28.6, -187.19], [24.2, -133.6, -185.48], [-51.0, 126.1, -185.3], [-112.4, -95.4, -176.36], [189.5, 16.0, -129.12], [43.6, 186.0, -127.83], [151.6, -120.8, -123.54], [-61.0, -190.8, -112.75], [-168.1, 116.2, -105.25], [-206.1, -20.7, -99.66], [155.1, 146.8, -85.04], [55.7, -211.6, -70.43], [-65.9, 208.7, -70.28], [-165.3, -149.7, -55.71], [220.3, 63.8, -15.32], [221.0, -61.9, -12.83], [61.4, 221.5, -2.26], [155.7, -169.1, -0.0], [-155.7, 169.1, 0.0], [-61.4, -221.5, 2.26], [-221.0, 61.9, 12.83], [-220.3, -63.8, 15.32], [165.3, 149.7, 55.71], [65.9, -208.7, 70.28], [-55.7, 211.6, 70.43], [-155.1, -146.8, 85.04], [206.1, 20.7, 99.66], [168.1, -116.2, 105.25], [61.0, 190.8, 112.75], [-151.6, 120.8, 123.54], [-43.6, -186.0, 127.83], [-189.5, -16.0, 129.12], [112.4, 95.4, 176.36], [51.0, -126.1, 185.3], [-24.2, 133.6, 185.48], [130.3, -28.6, 187.19], [-85.6, -87.9, 194.38], [-102.1, 36.5, 202.68], [16.5, 4.6, 229.22]],
  "edges": [[1, 0], [2, 0], [3, 0], [0, 4], [5, 0], [6, 0], [1, 2], [1, 4], [5, 2], [3, 5], [3, 6], [6, 4], [1, 7], [1, 9], [2, 7], [8, 2], [8, 5], [4, 9], [10, 4], [3, 11], [5, 11], [10, 6], [3, 12], [2, 13], [12, 6], [14, 4], [15, 5], [9, 7], [6, 16], [13, 7], [8, 13], [11, 12], [8, 15], [14, 9], [10, 14], [15, 11], [10, 16], [12, 16], [17, 7], [7, 18], [9, 18], [8, 19], [20, 9], [10, 22], [17, 13], [21, 11], [19, 13], [23, 11], [23, 12], [24, 12], [19, 15], [20, 14], [21, 15], [22, 14], [16, 22], [24, 16], [17, 18], [25, 13], [20, 18], [26, 14], [27, 15], [21, 23], [28, 16], [24, 23], [17, 25], [19, 25], [27, 19], [20, 26], [21, 27], [26, 22], [17, 29], [18, 29], [30, 18], [28, 22], [20, 30], [24, 28], [31, 19], [21, 32], [33, 22], [23, 32], [23, 34], [24, 34], [25, 29], [31, 25], [30, 26], [31, 27], [32, 27], [33, 26], [30, 29], [33, 28], [34, 28], [35, 25], [34, 32], [36, 26], [37, 27], [35, 29], [28, 39], [38, 29], [31, 35], [30, 36], [38, 30], [31, 37], [32, 37], [33, 36], [33, 39], [34, 39], [40, 32], [40, 34], [37, 35], [38, 35], [38, 36], [39, 36], [40, 37], [40, 39], [41, 35], [41, 36], [37, 41], [38, 41], [41, 39], [40, 41]]
};

export default function NetworkSphere({ className = '' }: { className?: string }) {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const W = 512, H = 512;
    const CX = W / 2, CY = H / 2;

    const COLOR = [100, 116, 139]; // #64748B
    const FOCAL = 900;      // camera distance -> perspective strength
    const TILT = -0.22;     // fixed tilt around X axis (radians) for a nicer viewing angle
    const SPEED = 0.5;      // radians per second

    function rotate(node: number[], theta: number) {
      let [x, y, z] = node;

      // fixed tilt around X axis
      const ct = Math.cos(TILT), st = Math.sin(TILT);
      const y1 = y * ct - z * st;
      const z1 = y * st + z * ct;
      const x1 = x;

      // continuous spin around Y axis
      const c = Math.cos(theta), s = Math.sin(theta);
      const x2 = x1 * c + z1 * s;
      const z2 = -x1 * s + z1 * c;
      const y2 = y1;

      return [x2, y2, z2];
    }

    function project(x: number, y: number, z: number) {
      const zoom = 1.05;
      const scale = (FOCAL / (FOCAL - z)) * zoom;
      return {
        sx: CX + x * scale,
        sy: CY + y * scale,
        scale
      };
    }

    function depthOpacity(z: number) {
      // z ranges roughly [-230, 230]
      const t = (z + 230) / 460;
      return 0.18 + t * 0.82;
    }

    let start: number | null = null;
    let animationId: number;

    function frame(ts: number) {
      if (start === null) start = ts;
      const t = (ts - start) / 1000;
      const theta = t * SPEED;

      // Ensure full cleanup
      ctx!.clearRect(0, 0, W, H);

      // rotate + project all nodes
      const pts = DATA.nodes.map(n => {
        const [x, y, z] = rotate(n, theta);
        const p = project(x, y, z);
        return { x, y, z, sx: p.sx, sy: p.sy, scale: p.scale };
      });

      // build drawable list: edges + nodes, sorted back-to-front by depth
      const drawables: any[] = [];
      DATA.edges.forEach(([a, b]) => {
        const pa = pts[a], pb = pts[b];
        const avgZ = (pa.z + pb.z) / 2;
        drawables.push({ type: 'edge', z: avgZ, a: pa, b: pb });
      });
      pts.forEach(p => {
        drawables.push({ type: 'node', z: p.z, p });
      });
      drawables.sort((d1, d2) => d1.z - d2.z);

      drawables.forEach(d => {
        if (d.type === 'edge') {
          const op = depthOpacity(d.z) * 0.55;
          const lw = 1.2 + ((d.a.scale + d.b.scale) / 2 - 0.75) * 6;
          ctx!.strokeStyle = `rgba(${COLOR[0]},${COLOR[1]},${COLOR[2]},${op.toFixed(3)})`;
          ctx!.lineWidth = Math.max(0.6, lw);
          ctx!.lineCap = 'round';
          ctx!.beginPath();
          ctx!.moveTo(d.a.sx, d.a.sy);
          ctx!.lineTo(d.b.sx, d.b.sy);
          ctx!.stroke();
        } else {
          const p = d.p;
          const op = depthOpacity(p.z);
          const r = Math.max(1.5, 9.5 * p.scale * 0.85);
          ctx!.fillStyle = `rgba(${COLOR[0]},${COLOR[1]},${COLOR[2]},${op.toFixed(3)})`;
          ctx!.beginPath();
          ctx!.arc(p.sx, p.sy, r, 0, Math.PI * 2);
          ctx!.fill();
        }
      });

      animationId = requestAnimationFrame(frame);
    }

    animationId = requestAnimationFrame(frame);

    return () => {
      cancelAnimationFrame(animationId);
    };
  }, []);

  return <canvas ref={canvasRef} width={512} height={512} className={className} />;
}
