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
  /** Скрыть из бокового меню, но оставить для хлебных крошек (детали, модалки) */
  hiddenFromMenu?: boolean;
  /** Кастомный лейбл для хлебных крошек (если отличается от label) */
  breadcrumbLabel?: string;
  /** Хэш для глубокой ссылки во вкладки (например, #users, #logs) */
  hash?: string;
}

import {
  LayoutDashboard,
  Box as BoxIcon,
  FileCode2,
  FileClock,
  Puzzle,
  FolderOpen,
  BookOpen,
  Palette,
  User,
  Users,
  Server,
  Terminal,
  RussianRuble,
  CreditCard,
} from 'lucide-react';

export const NAVIGATION: NavItem[] = [
  // ── Главные разделы (видно пользователям и админам) ──
  { key: 'dashboard',       label: 'Статистика',        icon: <LayoutDashboard size={20} />,   path: '/dashboard',           roles: ['user'] },
  { key: 'applications',    label: 'Приложения',        icon: <BoxIcon size={20} />,           path: '/applications',        roles: ['user'] },
  { key: 'env-configs',     label: 'Конфигурации .env', icon: <FileCode2 size={20} />,         path: '/env-configs',         roles: ['user'] },
  { key: 'integrations',    label: 'Интеграции',        icon: <Puzzle size={20} />,            path: '/integrations',        roles: ['user', 'admin'] },
  { key: 'files',           label: 'Файлы',             icon: <FolderOpen size={20} />,        path: '/files',               roles: ['user', 'admin'] },

  // ── Админ-панели (только админы, теперь в основном меню без раскрывающегося списка) ──
  { key: 'admin-users',      label: 'Пользователи',  icon: <Users size={20} />,   path: '/admin', hash: '#users',      roles: ['admin'] },
  { key: 'admin-containers', label: 'Контейнеры',    icon: <BoxIcon size={20} />, path: '/admin', hash: '#containers', roles: ['admin'] },
  { key: 'admin-nodes',      label: 'Серверы',       icon: <Server size={20} />,  path: '/admin', hash: '#servers',  roles: ['admin'] },
  { key: 'admin-requests',   label: 'Заявки',        icon: <FileClock size={20} />, path: '/admin', hash: '#requests', roles: ['admin'] },
  { key: 'admin-logs',       label: 'Логи',          icon: <Terminal size={20} />, path: '/admin', hash: '#logs',     roles: ['admin'] },
  { key: 'admin-pricing',    label: 'Тарифы',        icon: <RussianRuble size={20} />, path: '/admin', hash: '#pricing',  roles: ['admin'] },
  { key: 'admin-balances',   label: 'Балансы',       icon: <CreditCard size={20} />, path: '/admin', hash: '#balances', roles: ['admin'] },

  // ── Профиль / настройки (видно всем, но не в меню админа) ──
  { key: 'docs',         label: 'Документация',   icon: <BookOpen size={20} />,   path: '/docs',          roles: ['user'] },
  { key: 'appearance',   label: 'Персонализация', icon: <Palette size={20} />,    path: '/appearance',    roles: ['user', 'admin'] },
  { key: 'profile',      label: 'Профиль',        icon: <User size={20} />,       path: '/profile',       roles: ['user', 'admin'] },

  // ── Скрытые от меню, но нужные для хлебных крошек ──
  { key: 'app-new',      label: 'Создание приложения', path: '/applications/new', hiddenFromMenu: true, roles: ['user', 'admin'] },
  { key: 'app-details',  label: 'Детали приложения',   path: '/applications/:id', hiddenFromMenu: true, roles: ['user', 'admin'] },
  { key: 'container',    label: 'Детали контейнера',   path: '/containers/:id',   hiddenFromMenu: true, roles: ['user', 'admin'] },
  { key: 'node-details', label: 'Детали узла',         path: '/nodes/:id',        hiddenFromMenu: true, roles: ['admin'] },
  { key: 'volumes',      label: 'Тома',                path: '/volumes',          hiddenFromMenu: true, roles: ['admin'] },
];