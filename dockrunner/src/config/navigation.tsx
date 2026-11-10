import type { ReactNode } from 'react';

export interface NavItem {
  /** Уникальный ключ */
  key: string;
  /** Текст в меню / хлебных крошках */
  label: string;
  /** Иконка (опционально) */
  icon?: ReactNode;
  /** Путь — если есть, это линк; если нет — группа/заголовок секции */
  path?: string;
  /** Роли, которым виден пункт (пусто = всем авторизованным) */
  roles?: string[];
  /** Разрешения (permissions) — для более тонкого контроля */
  permissions?: string[];
  /** Вложенные пункты (подменю) */
  children?: NavItem[];
  /** Является ли заголовком секции (без линка, просто визуальный разделитель) */
  isGroupHeader?: boolean;
  /** Порядок сортировки внутри родителя */
  order?: number;
  /** Скрыть из бокового меню, но оставить для хлебных крошков (детали, модалки) */
  hiddenFromMenu?: boolean;
  /** Кастомный лейбл для хлебных крошек (если отличается от label) */
  breadcrumbLabel?: string;
}

import {
  LayoutDashboard,
  Box as BoxIcon,
  FileCode2,
  Puzzle,
  FolderOpen,
  BookOpen,
  Palette,
  User,
  ShieldCheck,
  Users,
  Server,
  Terminal,
  Settings,
} from 'lucide-react';

export const NAVIGATION: NavItem[] = [
  // ── Главные разделы (видно пользователям и админам) ──
  { key: 'dashboard',       label: 'Статистика',        icon: <LayoutDashboard size={20} />, path: '/dashboard',           roles: ['user', 'admin'] },
  { key: 'applications',    label: 'Приложения',        icon: <BoxIcon size={20} />,           path: '/applications',        roles: ['user', 'admin'] },
  { key: 'env-configs',     label: 'Конфигурации .env', icon: <FileCode2 size={20} />,       path: '/env-configs',         roles: ['user', 'admin'] },
  { key: 'integrations',    label: 'Интеграции',        icon: <Puzzle size={20} />,            path: '/integrations',        roles: ['user', 'admin'] },
  { key: 'files',           label: 'Файлы',             icon: <FolderOpen size={20} />,        path: '/files',               roles: ['user', 'admin'] },

  // ── Админ-секция (только админы) ──
  {
    key: 'admin-section',
    label: 'Администрирование',
    icon: <ShieldCheck size={20} />,
    roles: ['admin'],
    children: [
      { key: 'admin-users',   label: 'Пользователи',  icon: <Users size={20} />,   path: '/admin/users',   roles: ['admin'] },
      { key: 'admin-nodes',   label: 'Узлы',          icon: <Server size={20} />,  path: '/admin/nodes',   roles: ['admin'] },
      { key: 'admin-logs',    label: 'Логи',          icon: <Terminal size={20} />, path: '/admin/logs',    roles: ['admin'] },
      { key: 'admin-settings',label: 'Настройки',     icon: <Settings size={20} />, path: '/admin/settings',roles: ['admin'] },
    ]
  },

  // ── Профиль / настройки (видно всем, но не в меню админа) ──
  { key: 'docs',         label: 'Документация',   icon: <BookOpen size={20} />,   path: '/docs',          roles: ['user', 'admin'] },
  { key: 'appearance',   label: 'Персонализация', icon: <Palette size={20} />,    path: '/appearance',    roles: ['user', 'admin'] },
  { key: 'profile',      label: 'Профиль',        icon: <User size={20} />,       path: '/profile',       roles: ['user', 'admin'] },

  // ── Скрытые от меню, но нужные для хлебных крошек ──
  { key: 'app-new',      label: 'Создание приложения', path: '/applications/new', hiddenFromMenu: true, roles: ['user', 'admin'] },
  { key: 'app-details',  label: 'Детали приложения',   path: '/applications/:id', hiddenFromMenu: true, roles: ['user', 'admin'] },
  { key: 'container',    label: 'Детали контейнера',   path: '/containers/:id',   hiddenFromMenu: true, roles: ['user', 'admin'] },
  { key: 'node-details', label: 'Детали узла',         path: '/nodes/:id',        hiddenFromMenu: true, roles: ['admin'] },
  { key: 'volumes',      label: 'Тома',                path: '/volumes',          hiddenFromMenu: true, roles: ['admin'] },
];