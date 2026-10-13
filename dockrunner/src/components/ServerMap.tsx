import { useState, useCallback } from "react";
import {
  Box,
  Card,
  CardContent,
  Chip,
  Stack,
  Typography,
  useTheme,
  Tooltip,
  IconButton,
  Fade,
} from "@mui/material";
import {
  ComposableMap,
  Geographies,
  Geography,
  Marker,
  ZoomableGroup,
} from "@vnedyalk0v/react19-simple-maps";
import {
  CheckCircle2,
  CircleX,
  ZoomIn,
  ZoomOut,
  Maximize2,
  Move,
} from "lucide-react";
import { percent } from "../types/workerNode";
import type { WorkerNode } from "../types/workerNode";
import { timeAgo } from "../utils/time";
import { PingText } from "./Ping";
import type { Coordinates } from "@vnedyalk0v/react19-simple-maps";
import styles from "./ServerMap.module.scss";

const geoUrl = "https://unpkg.com/world-atlas@2.0.2/countries-110m.json";

const INITIAL_CENTER: Coordinates = [0, 20] as Coordinates;
const INITIAL_ZOOM = 1;
const MIN_ZOOM = 0.9;
const MAX_ZOOM = 8;
const ZOOM_STEP = 1.5;

interface ServerMapProps {
  nodes: WorkerNode[];
  selectedNodeId?: string;
  onSelectNode?: (node: WorkerNode) => void;
}

function getUsageBarColor(value: number): string {
  if (value >= 90) {
    return "error.main";
  }
  if (value >= 70) {
    return "warning.main";
  }
  return "success.main";
}

function UsageBar({
  label,
  value,
  text,
}: {
  label: string;
  value: number;
  text: string;
}) {
  const barColor = getUsageBarColor(value);
  const barWidth = Math.min(value, 100);

  return (
    <Box>
      <Stack direction="row" alignItems="baseline" sx={{ mb: 0.5 }}>
        <Typography variant="caption" color="text.secondary">
          {label}
        </Typography>
        <Typography variant="caption" sx={{ fontWeight: 600, whiteSpace: 'nowrap', marginLeft: 'auto', paddingLeft: 2, textAlign: 'right' }}>
          {text}
        </Typography>
      </Stack>
      <Box
        sx={{
          height: 5,
          borderRadius: 5,
          bgcolor: "action.hover",
          overflow: "hidden",
        }}
      >
        <Box
          sx={{
            height: "100%",
            width: `${barWidth}%`,
            bgcolor: barColor,
            borderRadius: 5,
          }}
        />
      </Box>
    </Box>
  );
}

function formatCpuText(node: WorkerNode): string {
  if (node.cpu.total > 0) {
    return `${node.cpu.used} / ${node.cpu.total} ядер`;
  }
  return `${node.cpu.used}%`;
}

function NodeTooltip({ node }: { node: WorkerNode }) {
  const cpu = percent(node.cpu.used, node.cpu.total);
  const ram = percent(node.memory.used, node.memory.total);
  const disk = percent(node.disk.used, node.disk.total);
  const offline = node.status === "offline";
  const cpuText = formatCpuText(node);

  let statusLabel = "Готов";
  if (offline) {
    statusLabel = "Офлайн";
  }

  let statusColor: "default" | "success" = "success";
  if (offline) {
    statusColor = "default";
  }

  const nodeIp = node.ip || '—';
  const networkText = `↓ ${node.net.down.toFixed(1)} · ↑ ${node.net.up.toFixed(1)} Мбит/с`;
  const cpuTextFull = node.cpu.mhz ? `${cpuText} · ${node.cpu.mhz} МГц` : cpuText;
  const memSpec = [node.memory.type, node.memory.speedMhz ? `${node.memory.speedMhz} МГц` : ''].filter(Boolean).join(' ');
  const ramTextFull = memSpec ? `${node.memory.used} / ${node.memory.total} ГБ · ${memSpec}` : `${node.memory.used} / ${node.memory.total} ГБ`;
  const diskTextFull = node.disk.type ? `${node.disk.used} / ${node.disk.total} ГБ · ${node.disk.type}` : `${node.disk.used} / ${node.disk.total} ГБ`;

  return (
    <Card sx={{ width: 280, pointerEvents: "none", boxShadow: 8 }}>
      <CardContent sx={{ p: 2, "&:last-child": { pb: 2 } }}>
        <Box sx={{ mb: 1.5 }}>
          <Typography variant="subtitle2" sx={{ fontWeight: 700 }}>
            {node.name}
          </Typography>
          <Typography variant="caption" color="text.secondary">
            {node.city}
          </Typography>
          {node.description && (
            <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 0.5 }}>
              {node.description}
            </Typography>
          )}
        </Box>
        <Stack direction="row" alignItems="center" sx={{ mb: 1.5 }}>
          <Typography variant="caption" color="text.secondary">
            Статус
          </Typography>
          <Box sx={{ marginLeft: 'auto', paddingLeft: 2 }}>
            <Chip
              size="small"
              label={statusLabel}
              color={statusColor}
              icon={offline ? <CircleX size={13} /> : <CheckCircle2 size={13} />}
            />
          </Box>
        </Stack>
        <Stack spacing={1.5}>
          <UsageBar
            label="CPU"
            value={cpu}
            text={cpuTextFull}
          />
          <UsageBar
            label="RAM"
            value={ram}
            text={ramTextFull}
          />
          <UsageBar
            label="Диск"
            value={disk}
            text={diskTextFull}
          />
        </Stack>
        <Stack
          direction="row"
          sx={{
            mt: 1.5,
            pt: 1.5,
            borderTop: "1px solid",
            borderColor: "divider",
          }}
        >
          <Typography variant="caption" color="text.secondary">
            Контейнеры
          </Typography>
          <Typography variant="caption" sx={{ fontWeight: 700, marginLeft: 'auto', paddingLeft: 2 }}>
            {node.containers}
          </Typography>
        </Stack>
        <Stack direction="row" alignItems="baseline" sx={{ mt: 1 }}>
          <Typography variant="caption" color="text.secondary">
            Heartbeat
          </Typography>
          <Typography variant="caption" sx={{ fontWeight: 700, marginLeft: 'auto', paddingLeft: 2, whiteSpace: 'nowrap' }}>
            {timeAgo(node.lastSeen)}
          </Typography>
        </Stack>
        <Stack direction="row" alignItems="baseline" sx={{ mt: 1 }}>
          <Typography variant="caption" color="text.secondary">
            Пинг
          </Typography>
          <Box sx={{ marginLeft: 'auto', paddingLeft: 2 }}>
            <PingText url={node.healthUrl} />
          </Box>
        </Stack>
        <Stack direction="row" alignItems="baseline" sx={{ mt: 1 }}>
          <Typography variant="caption" color="text.secondary">
            IP
          </Typography>
          <Typography variant="caption" sx={{ fontWeight: 700, marginLeft: 'auto', paddingLeft: 2, fontFamily: 'monospace', whiteSpace: 'nowrap' }}>
            {nodeIp}
          </Typography>
        </Stack>
        <Stack direction="row" alignItems="baseline" sx={{ mt: 1 }}>
          <Typography variant="caption" color="text.secondary">
            Сеть
          </Typography>
          <Typography variant="caption" sx={{ fontWeight: 700, marginLeft: 'auto', paddingLeft: 2, whiteSpace: 'nowrap' }}>
            {networkText}
          </Typography>
        </Stack>
      </CardContent>
    </Card>
  );
}

interface GeographyItem {
  rsmKey: string;
}

function getNodeCpuPercent(node: WorkerNode): number {
  if (node.cpu.total > 0) {
    return percent(node.cpu.used, node.cpu.total);
  }
  return Math.round(node.cpu.used);
}

export default function ServerMap({
  nodes,
  selectedNodeId,
  onSelectNode,
}: ServerMapProps) {
  const theme = useTheme();

  let mapColor = "#e0e0e0";
  let mapBorder = "#fff";
  if (theme.palette.mode === "dark") {
    mapColor = "#333";
    mapBorder = "#444";
  }

  const [position, setPosition] = useState<{
    coordinates: Coordinates;
    zoom: number;
  }>({
    coordinates: INITIAL_CENTER,
    zoom: INITIAL_ZOOM,
  });

  const handleZoomIn = useCallback(() => {
    setPosition((prev) => ({ ...prev, zoom: Math.min(prev.zoom * ZOOM_STEP, MAX_ZOOM) }));
  }, []);

  const handleZoomOut = useCallback(() => {
    setPosition((prev) => ({ ...prev, zoom: Math.max(prev.zoom / ZOOM_STEP, MIN_ZOOM) }));
  }, []);

  const handleReset = useCallback(() => {
    setPosition({ coordinates: INITIAL_CENTER, zoom: INITIAL_ZOOM });
  }, []);

  const handleMoveEnd = useCallback(
    (nextPosition: { coordinates: Coordinates; zoom: number }) => {
      setPosition(nextPosition);
    },
    [],
  );

  const zoomOutDisabled = position.zoom <= MIN_ZOOM + 0.05;
  const showHint = position.zoom === INITIAL_ZOOM;
  const cursor = onSelectNode ? "pointer" : "default";

  return (
    <div className={styles.wrapper}>
      <div className={styles.controls}>
        <Tooltip title="Приблизить" placement="left">
          <IconButton
            size="small"
            onClick={handleZoomIn}
            className={styles.controls__btn}
          >
            <ZoomIn size={16} />
          </IconButton>
        </Tooltip>
        <Tooltip title="Отдалить" placement="left">
          <IconButton
            size="small"
            onClick={handleZoomOut}
            disabled={zoomOutDisabled}
            className={styles.controls__btn}
          >
            <ZoomOut size={16} />
          </IconButton>
        </Tooltip>
        <div className={styles.controls__divider} />
        <Tooltip title="Сбросить вид" placement="left">
          <IconButton
            size="small"
            onClick={handleReset}
            className={styles.controls__btn}
          >
            <Maximize2 size={14} />
          </IconButton>
        </Tooltip>
      </div>

      <Fade in={showHint}>
        <div className={styles.hint}>
          <Move size={12} style={{ opacity: 0.7 }} />
          <span className={styles.hint__text}>
            Тяните для перемещения · Колесо для масштаба
          </span>
          <span className={styles.hint__zoom}>{position.zoom.toFixed(1)}×</span>
        </div>
      </Fade>

      <ComposableMap
        projection="geoMercator"
        style={{ width: "100%", height: "100%" }}
      >
        <ZoomableGroup
          center={position.coordinates}
          zoom={position.zoom}
          minZoom={MIN_ZOOM}
          maxZoom={MAX_ZOOM}
          onMoveEnd={handleMoveEnd}
        >
          <Geographies geography={geoUrl}>
            {(geographyProps: { geographies: GeographyItem[] }) =>
              geographyProps.geographies.map((geo) => (
                <Geography
                  key={(geo as GeographyItem & { rsmKey: string }).rsmKey}
                  geography={geo}
                  fill={mapColor}
                  stroke={mapBorder}
                  style={{
                    default: { outline: "none" },
                    hover: {
                      fill: theme.palette.action.hover,
                      outline: "none",
                    },
                    pressed: { outline: "none" },
                  }}
                />
              ))
            }
          </Geographies>
          {nodes.map((node) => {
            const selected = selectedNodeId === node.id;
            const cpu = getNodeCpuPercent(node);

            let statusColor = theme.palette.success.main;
            if (node.status === "offline") {
              statusColor = theme.palette.text.disabled;
            } else if (cpu >= 90) {
              statusColor = theme.palette.error.main;
            } else if (cpu >= 70) {
              statusColor = theme.palette.warning.main;
            }

            const markerRadius = selected ? 8 / position.zoom : 6 / position.zoom;
            const strokeWidth = selected ? 1.5 / position.zoom : 1 / position.zoom;
            const markerOpacity = node.status === "offline" ? 0.55 : 1;

            return (
              <Marker
                key={node.id}
                coordinates={node.coordinates as Coordinates}
              >
                <g
                  onClick={() => onSelectNode?.(node)}
                  style={{ cursor }}
                >
                  <circle
                    r={markerRadius}
                    fill={statusColor}
                    stroke={theme.palette.background.paper}
                    strokeWidth={strokeWidth}
                    opacity={markerOpacity}
                  />
                  {selected && (
                    <circle
                      r={12 / position.zoom}
                      fill="none"
                      stroke={theme.palette.primary.main}
                      strokeWidth={1 / position.zoom}
                      opacity={0.7}
                    />
                  )}
                  <Tooltip
                    title={<NodeTooltip node={node} />}
                    arrow
                    placement="top"
                    enterDelay={200}
                  >
                    <circle
                      r={12 / position.zoom}
                      fill="transparent"
                      stroke="transparent"
                    />
                  </Tooltip>
                  <title>{`${node.name} — ${node.city} — CPU ${cpu}%`}</title>
                </g>
              </Marker>
            );
          })}
        </ZoomableGroup>
      </ComposableMap>
    </div>
  );
}
