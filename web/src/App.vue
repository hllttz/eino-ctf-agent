<template>
  <div id="app">
    <nav class="app-nav">
      <router-link to="/">聊天</router-link>
      <router-link to="/knowledge">知识库</router-link>
      <router-link to="/skills">Skills</router-link>
    </nav>
    <main class="app-main">
      <router-view />
    </main>
  </div>
</template>

<script setup lang="ts">
// App 根组件
// TODO Phase 2.5: 完善布局（侧边栏 / 顶部导航 / 响应式设计）
</script>

<style scoped>
.app-nav {
  display: flex;
  gap: 1rem;
  padding: 1rem;
  border-bottom: 1px solid #e2e8f0;
}

.app-nav a {
  text-decoration: none;
  color: #4a5568;
  font-weight: 500;
}

.app-nav a.router-link-active {
  color: #3182ce;
}

.app-main {
  padding: 1rem;
}
</style>
