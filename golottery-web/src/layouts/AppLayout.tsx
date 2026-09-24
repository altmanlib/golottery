import { AppShell, Group, Text } from '@mantine/core';
import { Link, Outlet, useLocation } from 'react-router-dom';
import styles from './AppLayout.module.css';

export function AppLayout() {
  const location = useLocation();

  return (
    <AppShell header={{ height: 52 }} padding="md" className={styles.root}>
      <AppShell.Header>
        <Group h="100%" px="md" justify="space-between">
          <Group gap="lg">
            <Text fw={700}>golottery</Text>
            <Text component={Link} to="/console" c={location.pathname.startsWith('/console') ? 'brand' : 'dimmed'} size="sm">
              控制台
            </Text>
            <Text component={Link} to="/host" c="dimmed" size="sm">
              大屏
            </Text>
          </Group>
          <Text component={Link} to="/login" size="sm" c="dimmed">
            登录
          </Text>
        </Group>
      </AppShell.Header>
      <AppShell.Main>
        <Outlet />
      </AppShell.Main>
    </AppShell>
  );
}
