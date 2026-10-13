import { useRef, useState } from "react";
import {
  Box,
  Button,
  Card,
  CardContent,
  Stack,
  Typography,
  Divider,
  Slider,
  TextField,
  Chip,
  Grid,
  Switch,
  FormControlLabel,
} from "@mui/material";
import {
  Palette,
  Image as ImageIcon,
  RotateCcw,
  Save,
  Upload,
  Trash2,
  Eye,
} from "lucide-react";
import { useCustomTheme } from "../../theme/CustomThemeProvider";
import { PRESET_BACKGROUNDS } from "../../theme/colors";
import styles from "./Appearance.module.scss";

export default function Appearance() {
  const { config, updateConfig, reset } = useCustomTheme();
  const fileRef = useRef<HTMLInputElement>(null);
  const [urlInput, setUrlInput] = useState(config.backgroundImage || "");

  const handleFile = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = () => {
      const result = reader.result as string;
      updateConfig({ backgroundImage: result });
      setUrlInput(result);
    };
    reader.readAsDataURL(file);
  };

  const applyUrl = () => {
    if (!urlInput.trim()) {
      updateConfig({ backgroundImage: null });
      return;
    }
    updateConfig({ backgroundImage: urlInput.trim() });
  };

  const clearBackground = () => {
    updateConfig({ backgroundImage: null });
    setUrlInput("");
  };

  const selectPreset = (url: string) => {
    updateConfig({ backgroundImage: url });
    setUrlInput(url);
  };

  return (
    <Stack spacing={3} className={styles.page}>
      <Box>
        <Typography variant="h4" className={styles.header__title}>
          <Palette size={28} /> Персонализация
        </Typography>
        <Typography color="text.secondary">
          Настройте тему и фоновое изображение. Настройки сохраняются локально.
        </Typography>
      </Box>

      <Grid container spacing={3}>
        <Grid size={{ xs: 12, lg: 7 }}>
          <Stack spacing={3}>
            <Card>
              <CardContent>
                <Typography variant="h6" className={styles.sectionTitle}>
                  Режим темы
                </Typography>
                <Stack direction="row" spacing={2} alignItems="center">
                  <Chip
                    label="Светлая"
                    color={config.mode === "light" ? "primary" : "default"}
                    onClick={() => updateConfig({ mode: "light" })}
                    sx={{ cursor: "pointer" }}
                  />
                  <Chip
                    label="Тёмная"
                    color={config.mode === "dark" ? "primary" : "default"}
                    onClick={() => updateConfig({ mode: "dark" })}
                    sx={{ cursor: "pointer" }}
                  />
                  <FormControlLabel
                    control={
                      <Switch
                        checked={config.mode === "dark"}
                        onChange={(e) =>
                          updateConfig({
                            mode: e.target.checked ? "dark" : "light",
                          })
                        }
                      />
                    }
                    label={
                      config.mode === "dark" ? "Тёмная тема" : "Светлая тема"
                    }
                    sx={{ ml: 2 }}
                  />
                </Stack>
              </CardContent>
            </Card>

            <Card>
              <CardContent>
                <Typography variant="h6" className={styles.sectionTitle}>
                  Скругление углов
                </Typography>
                <Typography variant="body2" className={styles.label}>
                  Радиус: {config.borderRadius}px
                </Typography>
                <Slider
                  value={config.borderRadius}
                  min={0}
                  max={24}
                  step={1}
                  onChange={(_, v) =>
                    updateConfig({ borderRadius: v as number })
                  }
                  valueLabelDisplay="auto"
                />
              </CardContent>
            </Card>

            <Card>
              <CardContent>
                <Typography
                  variant="h6"
                  className={styles.sectionTitle}
                  sx={{ display: "flex", alignItems: "center", gap: 1 }}
                >
                  <ImageIcon size={20} /> Фоновое изображение
                </Typography>

                <Typography variant="body2" className={styles.label}>
                  Пресеты
                </Typography>
                <Box className={styles.presets}>
                  {PRESET_BACKGROUNDS.map((url) => (
                    <Box
                      key={url}
                      onClick={() => selectPreset(url)}
                      className={`${styles.presets__item} ${config.backgroundImage === url ? styles["presets__item--active"] : ""}`}
                    >
                      <img src={url} alt="" />
                    </Box>
                  ))}
                  <Box
                    onClick={clearBackground}
                    className={`${styles.presets__empty} ${!config.backgroundImage ? styles["presets__empty--active"] : ""}`}
                  >
                    <Trash2 size={16} />{" "}
                    <Typography variant="caption" sx={{ ml: 0.5 }}>
                      Нет
                    </Typography>
                  </Box>
                </Box>

                <Stack
                  direction={{ xs: "column", sm: "row" }}
                  spacing={1.5}
                  sx={{ mb: 2 }}
                >
                  <TextField
                    fullWidth
                    size="small"
                    label="URL изображения"
                    value={urlInput}
                    onChange={(e) => setUrlInput(e.target.value)}
                    placeholder="https://..."
                  />
                  <Button
                    variant="outlined"
                    onClick={applyUrl}
                    sx={{ whiteSpace: "nowrap" }}
                  >
                    Применить URL
                  </Button>
                </Stack>

                <input
                  ref={fileRef}
                  type="file"
                  accept="image/*"
                  hidden
                  onChange={handleFile}
                />
                <Stack direction="row" spacing={1.5} sx={{ mb: 3 }}>
                  <Button
                    variant="contained"
                    startIcon={<Upload size={16} />}
                    onClick={() => fileRef.current?.click()}
                  >
                    Загрузить с устройства
                  </Button>
                  {config.backgroundImage && (
                    <Button
                      variant="outlined"
                      color="error"
                      startIcon={<Trash2 size={16} />}
                      onClick={clearBackground}
                    >
                      Удалить фон
                    </Button>
                  )}
                </Stack>

                <Divider sx={{ mb: 2 }} />

                <Stack spacing={2.5}>
                  <Box>
                    <Typography variant="body2" className={styles.label}>
                      Прозрачность фона:{" "}
                      {(config.backgroundOpacity * 100).toFixed(0)}%
                    </Typography>
                    <Slider
                      value={config.backgroundOpacity}
                      min={0}
                      max={1}
                      step={0.05}
                      onChange={(_, v) =>
                        updateConfig({ backgroundOpacity: v as number })
                      }
                      valueLabelDisplay="auto"
                    />
                  </Box>
                  <Box>
                    <Typography variant="body2" className={styles.label}>
                      Размытие: {config.backgroundBlur}px
                    </Typography>
                    <Slider
                      value={config.backgroundBlur}
                      min={0}
                      max={20}
                      step={1}
                      onChange={(_, v) =>
                        updateConfig({ backgroundBlur: v as number })
                      }
                      valueLabelDisplay="auto"
                    />
                  </Box>
                  <Box>
                    <Typography
                      variant="body2"
                      className={styles.label}
                    >
                      Прозрачность затемнения:{" "}
                      {(config.overlayOpacity * 100).toFixed(0)}%
                    </Typography>
                    <Slider
                      value={config.overlayOpacity}
                      min={0}
                      max={1}
                      step={0.05}
                      onChange={(_, v) =>
                        updateConfig({ overlayOpacity: v as number })
                      }
                      valueLabelDisplay="auto"
                    />
                  </Box>
                </Stack>
              </CardContent>
            </Card>

            <Stack
              direction="row"
              spacing={2}
              className={styles.previewCard__actions}
            >
              <Button
                variant="outlined"
                startIcon={<RotateCcw size={16} />}
                onClick={reset}
              >
                Сбросить
              </Button>
              <Button
                variant="contained"
                startIcon={<Save size={16} />}
                onClick={() => window.location.reload()}
              >
                Сохранить (авто)
              </Button>
            </Stack>
          </Stack>
        </Grid>

        <Grid size={{ xs: 12, lg: 5 }}>
          <Card className={styles.previewCard}>
            <CardContent>
              <Typography
                variant="h6"
                className={styles.sectionTitle}
                sx={{ display: "flex", alignItems: "center", gap: 1 }}
              >
                <Eye size={18} /> Превью
              </Typography>
              <Box
                className={styles.previewCard__viewport}
                sx={{ borderRadius: `${config.borderRadius}px` }}
              >
                {config.backgroundImage && (
                  <Box
                    className={styles.previewCard__bg}
                    sx={{
                      backgroundImage: `url("${config.backgroundImage}")`,
                      opacity: config.backgroundOpacity,
                      filter: `blur(${config.backgroundBlur}px)`,
                    }}
                  />
                )}
                {config.backgroundImage && (
                  <Box
                    className={styles.previewCard__overlay}
                    sx={{ opacity: config.overlayOpacity }}
                  />
                )}
                <Box className={styles.previewCard__content}>
                  <Box className={styles.previewCard__inner}>
                    <Typography sx={{ fontWeight: 700 }}>
                      Пример карточки
                    </Typography>
                    <Typography
                      variant="body2"
                      color="text.secondary"
                      sx={{ mt: 0.5 }}
                    >
                      Так будет выглядеть контент поверх выбранного фона.
                    </Typography>
                  </Box>
                  <Box
                    className={styles.previewCard__inner}
                    sx={{
                      display: "flex",
                      justifyContent: "space-between",
                      alignItems: "center",
                    }}
                  >
                    <Typography variant="body2" sx={{ fontWeight: 600 }}>
                      Метрика
                    </Typography>
                    <Typography variant="body2" sx={{ fontWeight: 700 }}>
                      78%
                    </Typography>
                  </Box>
                </Box>
              </Box>

              <Divider sx={{ my: 2 }} />
              <Typography variant="body2" color="text.secondary">
                Загрузите своё изображение или выберите пресет. Настройте
                прозрачность и размытие для читаемости контента.
              </Typography>
            </CardContent>
          </Card>
        </Grid>
      </Grid>
    </Stack>
  );
}
