import { NAVIGATION, type NavItem } from '../config/navigation';

/** Плоский список всех элементов (для поиска по path/key) */
export function flattenNavigation(items: NavItem[] = NAVIGATION): NavItem[] {
  return items.flatMap(item => [
    item,
    ...(item.children ? flattenNavigation(item.children) : [])
  ]);
}

/** Проверить, виден ли элемент для роли пользователя */
export function isItemVisible(item: NavItem, userRole?: string): boolean {
  if (!userRole) return false;
  if (!item.roles || item.roles.length === 0) return true;
  return item.roles.includes(userRole);
}

/** Найти элемент по точному path или по паттерну с :param */
export function findNavItem(pathname: string, items: NavItem[] = NAVIGATION): NavItem | undefined {
  const all = flattenNavigation(items);

  // 1. Точное совпадение
  let found = all.find(item => item.path === pathname);
  if (found) return found;

  // 2. Паттерн с параметрами (/applications/:id)
  for (const item of all) {
    if (!item.path) continue;
    const pattern = '^' + item.path.replace(/:[^/]+/g, '[^/]+') + '$';
    if (new RegExp(pattern).test(pathname)) return item;
  }

  return undefined;
}

/** Найти родителя элемента в дереве */
export function findParent(items: NavItem[], childKey: string): NavItem | undefined {
  for (const item of items) {
    if (item.children?.some(c => c.key === childKey)) return item;
    if (item.children) {
      const found = findParent(item.children, childKey);
      if (found) return found;
    }
  }
  return undefined;
}

/** Построить цепочку хлебных крошек для pathname */
export function getBreadcrumbs(pathname: string, items: NavItem[] = NAVIGATION): NavItem[] {
  const item = findNavItem(pathname, items);
  if (!item) return [];

  const chain: NavItem[] = [];
  let current: NavItem | undefined = item;

  while (current) {
    chain.unshift(current);
    current = findParent(items, current.key);
  }

  // Фильтруем: только с path, не скрытые из меню, не group headers
  return chain.filter(c => c.path && !c.hiddenFromMenu && !c.isGroupHeader);
}

/** Фильтр видимых элементов для текущего пользователя (рекурсивно) */
export function filterVisible(items: NavItem[], userRole?: string): NavItem[] {
  return items
    .filter(item => isItemVisible(item, userRole))
    .map(item => ({
      ...item,
      children: item.children ? filterVisible(item.children, userRole) : undefined
    }))
    .filter(item => item.children?.length || item.path); // убрать пустые группы
}

/** Проверить, активен ли путь (точное совпадение или префикс) */
export function isNavActive(currentPath: string, itemPath?: string): boolean {
  if (!itemPath) return false;
  if (currentPath === itemPath) return true;
  return currentPath.startsWith(itemPath + '/');
}

/** Получить заголовок страницы для pathname */
export function getPageTitle(pathname: string, items: NavItem[] = NAVIGATION): string {
  const item = findNavItem(pathname, items);
  if (item) return item.breadcrumbLabel || item.label;

  // Fallback для путей, которых нет в конфиге
  if (pathname.startsWith('/docs')) return 'Документация';
  if (pathname.startsWith('/applications/new')) return 'Создание приложения';
  if (pathname.startsWith('/applications/')) return 'Детали приложения';
  if (pathname.startsWith('/containers/')) return 'Детали контейнера';
  if (pathname.startsWith('/nodes/')) return 'Детали узла';
  if (pathname.startsWith('/login')) return 'Вход';
  if (pathname.startsWith('/volumes')) return 'Тома';
  return 'Обзор';
}

/** Получить видимые пункты меню для рендера в сайдбаре (плоский список групп и элементов) */
export function getVisibleMenuItems(items: NavItem[], userRole?: string): NavItem[] {
  const visible = filterVisible(items, userRole);
  return visible.filter(item => !item.hiddenFromMenu);
}