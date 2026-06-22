"use client";

interface SparklineProps {
  data: number[];
  width?: number;
  height?: number;
  color?: string;
  fillColor?: string;
  max?: number;
  className?: string;
}

/**
 * Minimalistischer SVG-Sparkline-Chart.
 * Zeigt einen Datenverlauf ohne Achsen oder Labels.
 */
export function Sparkline({
  data,
  width = 120,
  height = 32,
  color = "var(--brand-l)",
  fillColor,
  max,
  className,
}: SparklineProps) {
  if (data.length < 2) {
    return <svg width={width} height={height} className={className} />;
  }

  const maxVal = max ?? Math.max(...data, 1);
  const minVal = 0;
  const range = maxVal - minVal || 1;

  const points = data.map((val, i) => {
    const x = (i / (data.length - 1)) * width;
    const y = height - ((val - minVal) / range) * height;
    return `${x},${y}`;
  });

  const linePath = `M ${points.join(" L ")}`;
  const fillPath = `${linePath} L ${width},${height} L 0,${height} Z`;
  const fill = fillColor ?? color.replace(")", ", 0.15)").replace("var(", "color-mix(in srgb, ").replace(", 0.15)", " 15%, transparent)");

  return (
    <svg
      width={width}
      height={height}
      viewBox={`0 0 ${width} ${height}`}
      preserveAspectRatio="none"
      className={className}
      style={{ overflow: "visible" }}
    >
      {/* Fill */}
      <path d={fillPath} fill={fill} />
      {/* Line */}
      <path
        d={linePath}
        fill="none"
        stroke={color}
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      {/* Letzter Punkt hervorgehoben */}
      {data.length > 0 && (
        <circle
          cx={(data.length - 1) / (data.length - 1) * width}
          cy={height - ((data[data.length - 1] - minVal) / range) * height}
          r="2"
          fill={color}
        />
      )}
    </svg>
  );
}
