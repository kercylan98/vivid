// 主要JavaScript文件
// 使用严格模式
'use strict';

// 全局应用对象
const VividApp = {
    // 应用状态
    state: {
        isLoaded: false,
        currentSection: '',
        scrollPosition: 0,
        isMobileMenuOpen: false
    },

    // 配置
    config: {
        animationDelay: 100,
        scrollOffset: 70,
        typewriterSpeed: 20,  // 进一步加快打字速度
        countUpDuration: 2000
    },

    // 初始化应用
    init() {
        this.bindEvents();
        this.initializeComponents();
        this.startAnimations();
        this.state.isLoaded = true;
    },

    // 绑定事件
    bindEvents() {
        // 导航点击事件
        Utils.events.on(document, 'click', this.handleNavigation.bind(this));

        // Logo点击回到顶部
        Utils.events.on('#logo-home', 'click', (e) => {
            e.preventDefault();
            window.scrollTo({top: 0, behavior: 'smooth'});
        });

        // 滚动事件（使用节流）
        Utils.events.on(window, 'scroll', Utils.throttle(this.handleScroll.bind(this), 16));

        // 窗口大小改变事件（使用防抖）
        Utils.events.on(window, 'resize', Utils.debounce(this.handleResize.bind(this), 250));

        // 移动菜单切换
        Utils.events.on('.mobile-menu-toggle', 'click', this.toggleMobileMenu.bind(this));

        // 统计数字动画触发
        this.observeElements();
    },

    // 处理导航
    handleNavigation(event) {
        const target = event.target.closest('a[href^="#"]');
        if (!target) return;

        // 立即阻止默认行为，提高响应速度
        event.preventDefault();

        // 如果是外部链接，直接返回让默认行为处理
        if (target.classList.contains('external')) {
            window.open(target.href, '_blank', 'noopener,noreferrer');
            return;
        }

        const href = target.getAttribute('href');
        if (href === '#') return;

        const targetElement = document.querySelector(href);
        if (targetElement) {
            // 使用更快的滚动响应
            Utils.smoothScrollTo(targetElement, {
                offset: this.config.scrollOffset,
                duration: 400,  // 进一步减少动画时间
                easing: 'easeInOutCubic'
            });

            this.closeMobileMenu();
        }
    },

    // 处理滚动
    handleScroll() {
        const scrollTop = window.pageYOffset;
        this.state.scrollPosition = scrollTop;

        // 更新导航栏状态
        this.updateNavbar(scrollTop);

        // 更新当前区域
        this.updateCurrentSection();

        // 视差效果
        this.updateParallax(scrollTop);
    },

    // 处理窗口大小改变
    handleResize() {
        // 关闭移动菜单
        this.closeMobileMenu();

        // 重新计算动画
        this.recalculateAnimations();
    },

    // 更新导航栏
    updateNavbar(scrollTop) {
        const navbar = document.querySelector('.navbar');
        if (!navbar) return;

        if (scrollTop > 100) {
            Utils.dom.addClass(navbar, 'scrolled');
        } else {
            Utils.dom.removeClass(navbar, 'scrolled');
        }
    },

    // 更新当前区域
    updateCurrentSection() {
        const sections = document.querySelectorAll('section[id]');
        let currentSection = '';

        sections.forEach(section => {
            const rect = section.getBoundingClientRect();
            if (rect.top <= 100 && rect.bottom >= 100) {
                currentSection = section.id;
            }
        });

        if (currentSection !== this.state.currentSection) {
            this.state.currentSection = currentSection;
            this.updateActiveNavLink(currentSection);
        }
    },

    // 更新活跃导航链接
    updateActiveNavLink(sectionId) {
        const navLinks = document.querySelectorAll('.nav-link');
        navLinks.forEach(link => {
            const href = link.getAttribute('href');
            if (href === `#${sectionId}`) {
                Utils.dom.addClass(link, 'active');
            } else {
                Utils.dom.removeClass(link, 'active');
            }
        });
    },

    // 视差效果
    updateParallax(scrollTop) {
        const parallaxElements = document.querySelectorAll('[data-parallax]');
        parallaxElements.forEach(element => {
            const speed = parseFloat(element.dataset.parallax) || 0.5;
            const yPos = -(scrollTop * speed);
            element.style.transform = `translateY(${yPos}px)`;
        });
    },

    // 切换移动菜单
    toggleMobileMenu() {
        const navbar = document.querySelector('.navbar');
        const toggle = document.querySelector('.mobile-menu-toggle');

        this.state.isMobileMenuOpen = !this.state.isMobileMenuOpen;

        if (this.state.isMobileMenuOpen) {
            Utils.dom.addClass(navbar, 'menu-open');
            Utils.dom.addClass(toggle, 'active');
        } else {
            Utils.dom.removeClass(navbar, 'menu-open');
            Utils.dom.removeClass(toggle, 'active');
        }
    },

    // 关闭移动菜单
    closeMobileMenu() {
        if (this.state.isMobileMenuOpen) {
            this.toggleMobileMenu();
        }
    },

    // 初始化组件
    initializeComponents() {
        this.initHeroCode();
        this.initFeatures();
        this.initArchitecture();
        this.initInstallationSteps();
        this.initExamples();
        this.initParticles();
    },

    // 初始化英雄区域代码展示
    initHeroCode() {
        const codeElement = document.getElementById('hero-code');
        if (!codeElement) return;

        const codeContent = `package main

import (
  "fmt"
  "github.com/kercylan98/vivid/src/vivid"
)

func main() {
  system := vivid.NewActorSystem()
  system.StartP()
  defer system.StopP()
  
  ref := system.ActorOf(func() vivid.Actor {
    return vivid.ActorFN(func(ctx vivid.ActorContext) {
      fmt.Println("收到消息:", ctx.Message())
    })
  })
  
  system.Tell("Hello Vivid!")
}`;

        this.typewriterEffect(codeElement, codeContent, this.config.typewriterSpeed);
    },

    // 打字机效果
    typewriterEffect(element, text, speed) {
        element.textContent = '';
        let index = 0;

        const type = () => {
            if (index < text.length) {
                element.textContent += text.charAt(index);
                index++;
                setTimeout(type, speed);
            }
        };

        // 延迟开始
        setTimeout(type, 1500);
    },

    // 初始化特性展示
    initFeatures() {
        const featuresData = [
            {
                icon: 'M13 2L3 14h9l-1 8 10-12h-9l1-8z',
                title: '简洁高效',
                description: '基于消息传递的并发模型，避免共享状态的复杂性，提供清晰的编程体验。',
                tags: ['消息传递', '无锁设计', '类型安全']
            },
            {
                icon: 'M20 13c0 5-3.5 7.5-7.66 8.95a1 1 0 01-.67-.01C7.5 20.5 4 18 4 13V6a1 1 0 01.5-.87l8-4.5a1 1 0 011 0l8 4.5A1 1 0 0120 6v7z',
                title: '容错机制',
                description: '内置监督策略和重启机制，当Actor发生异常时自动恢复，确保系统稳定运行。',
                tags: ['监督树', '故障隔离', '自动恢复']
            },
            {
                icon: 'M21 16V8a2 2 0 00-1-1.73l-7-4a2 2 0 00-2 0l-7 4A2 2 0 003 8v8a2 2 0 001 1.73l7 4a2 2 0 002 0l7-4A2 2 0 0021 16z',
                title: '扩展性',
                description: '支持Actor的动态创建和销毁，可根据负载情况灵活调整系统规模。',
                tags: ['动态扩容', '负载均衡', '资源管理']
            },
            {
                icon: 'M9 19c-5 1.5-5-2.5-7-3m14 6v-3.87a3.37 3.37 0 00-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0020 4.77 5.07 5.07 0 0019.91 1S18.73.65 16 2.48a13.38 13.38 0 00-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 005 4.77a5.44 5.44 0 00-1.5 3.78c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 009 18.13V22',
                title: '开源友好',
                description: 'MIT许可证开源，活跃的社区支持，欢迎贡献代码和反馈问题。',
                tags: ['MIT协议', '社区驱动', '持续改进']
            },
            {
                icon: 'M9 12l2 2 4-4M7.835 4.697a3.42 3.42 0 001.946-.806 3.42 3.42 0 014.438 0 3.42 3.42 0 001.946.806 3.42 3.42 0 013.138 3.138 3.42 3.42 0 00.806 1.946 3.42 3.42 0 010 4.438 3.42 3.42 0 00-.806 1.946 3.42 3.42 0 01-3.138 3.138 3.42 3.42 0 00-1.946.806 3.42 3.42 0 01-4.438 0 3.42 3.42 0 00-1.946-.806 3.42 3.42 0 01-3.138-3.138 3.42 3.42 0 00-.806-1.946 3.42 3.42 0 010-4.438 3.42 3.42 0 00.806-1.946 3.42 3.42 0 013.138-3.138z',
                title: '监控支持',
                description: '内置指标收集和监控功能，提供系统运行状态的实时观察能力。',
                tags: ['指标收集', '实时监控', '性能分析']
            },
            {
                icon: 'M10 20l4-16m-4 4l4 4-4 4',
                title: '简易集成',
                description: '标准的Go模块，易于集成到现有项目中，提供清晰的API设计。',
                tags: ['Go模块', '简洁API', '易于使用']
            }
        ];

        this.renderFeatures(featuresData);
    },

    // 渲染特性卡片
    renderFeatures(featuresData) {
        const container = document.getElementById('features-grid');
        if (!container) return;

        container.innerHTML = '';

        featuresData.forEach((feature, index) => {
            const card = this.createFeatureCard(feature, index);
            container.appendChild(card);
        });
    },

    // 创建特性卡片
    createFeatureCard(feature, index) {
        const card = Utils.dom.create('div', {
            className: 'feature-card',
            'data-aos': 'slide-up',
            'data-aos-delay': index * 100
        });

        card.innerHTML = `
      <div class="feature-icon">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor">
          <path d="${feature.icon}"/>
        </svg>
      </div>
      <h3 class="feature-title">${feature.title}</h3>
      <p class="feature-description">${feature.description}</p>
      <div class="feature-tags">
        ${feature.tags.map(tag => `<span class="feature-tag">${tag}</span>`).join('')}
      </div>
    `;

        return card;
    },

    // 初始化架构图
    initArchitecture() {
        const container = document.getElementById('architecture-svg');
        if (!container) return;

        // 清空容器
        container.innerHTML = '';

        // 设置SVG尺寸
        container.setAttribute('viewBox', '0 0 900 700');

        // 创建渐变定义
        this.createArchitectureDefs(container);

        // 创建架构层级数据
        // 颜色配置说明：
        // - color: 图层背景渐变和连接线颜色
        // - textColor: 图层主标题文字颜色
        // - subTextColor: 图层副标题文字颜色
        // - featureBorderColor: 特性标签边框颜色
        // - featureTextColor: 特性标签文字颜色
        const layers = [
            {
                name: 'Application Layer',
                description: '应用层',
                y: 120,
                width: 700,
                color: '#00d4ff',
                textColor: '#ffffff',
                subTextColor: '#eeefff',
                featureBorderColor: '#00b4d8',
                featureTextColor: '#caf0f8',
                icon: 'M13 2L3 14h9l-1 8 10-12h-9l1-8z',
                features: ['业务逻辑', 'API接口', '用户交互']
            },
            {
                name: 'Actor System Layer',
                description: 'Actor系统层',
                y: 220,
                width: 650,
                color: '#6c5ce7',
                textColor: '#ffffff',
                subTextColor: '#eeefff',
                featureBorderColor: '#5a52d5',
                featureTextColor: '#e0ddff',
                icon: 'M21 16V8a2 2 0 00-1-1.73l-7-4a2 2 0 00-2 0l-7 4A2 2 0 003 8v8a2 2 0 001 1.73l7 4a2 2 0 002 0l7-4A2 2 0 0021 16z',
                features: ['Actor管理', '生命周期', '监督策略']
            },
            {
                name: 'Message Passing Layer',
                description: '消息传递层',
                y: 320,
                width: 570,
                color: '#a29bfe',
                textColor: '#ffffff',
                subTextColor: '#eeefff',
                featureBorderColor: '#9081f5',
                featureTextColor: '#f0efff',
                icon: 'M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z',
                features: ['消息路由', '异步通信', '背压处理']
            },
            {
                name: 'Monitoring Layer',
                description: '监控层',
                y: 420,
                width: 500,
                color: '#fd79a8',
                textColor: '#ffffff',
                subTextColor: '#eeefff',
                featureBorderColor: '#fb6090',
                featureTextColor: '#ffe8f0',
                icon: 'M9 12l2 2 4-4M7.835 4.697a3.42 3.42 0 001.946-.806 3.42 3.42 0 014.438 0 3.42 3.42 0 001.946.806 3.42 3.42 0 013.138 3.138 3.42 3.42 0 00.806 1.946 3.42 3.42 0 010 4.438 3.42 3.42 0 00-.806 1.946 3.42 3.42 0 01-3.138 3.138 3.42 3.42 0 00-1.946.806 3.42 3.42 0 01-4.438 0 3.42 3.42 0 00-1.946-.806 3.42 3.42 0 01-3.138-3.138 3.42 3.42 0 00-.806-1.946 3.42 3.42 0 010-4.438 3.42 3.42 0 00.806-1.946 3.42 3.42 0 013.138-3.138z',
                features: ['性能指标', '健康检查', '实时监控']
            },
            {
                name: 'Runtime Layer',
                description: '运行时层',
                y: 520,
                width: 420,
                color: '#00b894',
                textColor: '#ffffff',
                subTextColor: '#eeefff',
                featureBorderColor: '#00997a',
                featureTextColor: '#c7f9e8',
                icon: 'M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z',
                features: ['Go Runtime', '协程池', '内存管理']
            }
        ];

        // 绘制连接线
        this.drawConnections(container, layers);

        // 绘制层级
        layers.forEach((layer, index) => {
            this.drawArchitectureLayer(container, layer, index);
        });

        // 添加标题
        this.addArchitectureTitle(container);
    },

    // 创建SVG定义
    createArchitectureDefs(container) {
        const defs = Utils.createSVGElement('defs');

        // 渐变定义
        const gradients = [
            {id: 'layerGradient1', colors: ['#00d4ff', '#0099cc']},
            {id: 'layerGradient2', colors: ['#6c5ce7', '#5a4fcf']},
            {id: 'layerGradient3', colors: ['#a29bfe', '#8b7cf8']},
            {id: 'layerGradient4', colors: ['#fd79a8', '#e84393']},
            {id: 'layerGradient5', colors: ['#00b894', '#00a085']}
        ];

        gradients.forEach(grad => {
            const gradient = Utils.createSVGElement('linearGradient', {
                id: grad.id,
                x1: '0%',
                y1: '0%',
                x2: '100%',
                y2: '100%'
            });

            const stop1 = Utils.createSVGElement('stop', {
                offset: '0%',
                'stop-color': grad.colors[0]
            });
            const stop2 = Utils.createSVGElement('stop', {
                offset: '100%',
                'stop-color': grad.colors[1]
            });

            gradient.appendChild(stop1);
            gradient.appendChild(stop2);
            defs.appendChild(gradient);
        });

        // 滤镜定义
        const filter = Utils.createSVGElement('filter', {
            id: 'glow',
            x: '-50%',
            y: '-50%',
            width: '200%',
            height: '200%'
        });

        const feGaussianBlur = Utils.createSVGElement('feGaussianBlur', {
            stdDeviation: '3',
            result: 'coloredBlur'
        });

        const feMerge = Utils.createSVGElement('feMerge');
        const feMergeNode1 = Utils.createSVGElement('feMergeNode', {in: 'coloredBlur'});
        const feMergeNode2 = Utils.createSVGElement('feMergeNode', {in: 'SourceGraphic'});

        feMerge.appendChild(feMergeNode1);
        feMerge.appendChild(feMergeNode2);
        filter.appendChild(feGaussianBlur);
        filter.appendChild(feMerge);
        defs.appendChild(filter);

        container.appendChild(defs);
    },

    // 绘制连接线
    drawConnections(container, layers) {
        for (let i = 0; i < layers.length - 1; i++) {
            const current = layers[i];
            const next = layers[i + 1];

            // 多条连接线
            for (let j = 0; j < 3; j++) {
                const offsetX = (j - 1) * 60;
                const startX = 450 + offsetX;
                const startY = current.y + 40;
                const endX = 450 + offsetX;
                const endY = next.y - 20;

                const path = Utils.createSVGElement('path', {
                    d: `M ${startX} ${startY} Q ${startX + (j - 1) * 20} ${(startY + endY) / 2} ${endX} ${endY}`,
                    stroke: current.color,
                    'stroke-width': 2,
                    fill: 'none',
                    opacity: 0.6,
                    class: 'architecture-connection',
                    'stroke-dasharray': '5,5'
                });

                // 添加动画
                const animate = Utils.createSVGElement('animate', {
                    attributeName: 'stroke-dashoffset',
                    from: '10',
                    to: '0',
                    dur: '2s',
                    repeatCount: 'indefinite'
                });

                path.appendChild(animate);
                container.appendChild(path);
            }
        }
    },

    // 绘制架构层
    drawArchitectureLayer(container, layer, index) {
        const x = (900 - layer.width) / 2;
        const gradientId = `layerGradient${index + 1}`;

        // 主要矩形
        const rect = Utils.createSVGElement('rect', {
            x: x,
            y: layer.y - 30,
            width: layer.width,
            height: 60,
            rx: 16,
            fill: `url(#${gradientId})`,
            stroke: layer.color,
            'stroke-width': 2,
            opacity: 0.9,
            filter: 'url(#glow)',
            class: 'architecture-layer-rect'
        });

        // 图标背景
        const iconBg = Utils.createSVGElement('circle', {
            cx: x + 40,
            cy: layer.y,
            r: 20,
            fill: 'rgba(255, 255, 255, 0.1)',
            stroke: 'rgba(255, 255, 255, 0.3)',
            'stroke-width': 1
        });

        // 图标容器 - 正确居中在圆形背景中
        const iconContainer = Utils.createSVGElement('g', {
            transform: `translate(${x + 40}, ${layer.y})`
        });

        // 图标 - 使用24x24的viewBox，居中对齐
        const icon = Utils.createSVGElement('svg', {
            x: -12,
            y: -12,
            width: 24,
            height: 24,
            viewBox: '0 0 24 24',
            fill: 'none',
            stroke: 'white',
            'stroke-width': 1.5,
            'stroke-linecap': 'round',
            'stroke-linejoin': 'round'
        });

        const iconPath = Utils.createSVGElement('path', {
            d: layer.icon
        });

        icon.appendChild(iconPath);
        iconContainer.appendChild(icon);

        // 主标题
        const title = Utils.createSVGElement('text', {
            x: x + 80,
            y: layer.y - 8,
            fill: layer.textColor,
            'font-family': 'SF Pro Display, -apple-system, sans-serif',
            'font-size': '16',
            'font-weight': '600'
        });
        title.textContent = layer.name;

        // 副标题
        const subtitle = Utils.createSVGElement('text', {
            x: x + 80,
            y: layer.y + 10,
            fill: layer.subTextColor,
            'font-family': 'SF Pro Display, -apple-system, sans-serif',
            'font-size': '12',
            'font-weight': '400'
        });
        subtitle.textContent = layer.description;

        // 先添加layer的主要元素（这些会在底层）
        const group = Utils.createSVGElement('g', {
            class: 'architecture-layer-group',
            'data-layer': index
        });

        group.appendChild(rect);
        group.appendChild(iconBg);
        group.appendChild(iconContainer);
        group.appendChild(title);
        group.appendChild(subtitle);

        container.appendChild(group);

        // 最后添加特性标签（这些会在最上层，不会被覆盖）
        layer.features.forEach((feature, idx) => {
            const tagX = x + layer.width - 220 + (idx * 70);
            const tagY = layer.y - 10;

            // 标签背景
            const tag = Utils.createSVGElement('rect', {
                x: tagX,
                y: tagY,
                width: 65,
                height: 20,
                rx: 10,
                fill: `rgba(255, 255, 255, 0.2)`,
                stroke: layer.featureBorderColor,
                'stroke-width': 2,
                class: 'feature-tag'
            });

            // 标签内部高亮
            const tagHighlight = Utils.createSVGElement('rect', {
                x: tagX + 1,
                y: tagY + 1,
                width: 63,
                height: 18,
                rx: 9,
                fill: layer.featureBorderColor,
                opacity: 0.2,
                class: 'feature-tag-highlight'
            });

            // 标签文字
            const tagText = Utils.createSVGElement('text', {
                x: tagX + 32.5,
                y: tagY + 13,
                fill: layer.featureTextColor,
                'font-family': 'SF Pro Display, -apple-system, sans-serif',
                'font-size': '9',
                'font-weight': '300',
                'text-anchor': 'middle',
                class: 'feature-tag-text'
            });
            tagText.textContent = feature;

            // 为标签添加悬停动画组
            const tagGroup = Utils.createSVGElement('g', {
                class: 'feature-tag-group',
                'data-feature': feature,
                style: `--tag-index: ${idx}`
            });

            tagGroup.appendChild(tag);
            tagGroup.appendChild(tagHighlight);
            tagGroup.appendChild(tagText);

            // 添加入场动画
            const animateOpacity = Utils.createSVGElement('animate', {
                attributeName: 'opacity',
                from: '0',
                to: '1',
                dur: '0.8s',
                begin: `${idx * 0.2}s`,
                fill: 'freeze'
            });

            const animateTransform = Utils.createSVGElement('animateTransform', {
                attributeName: 'transform',
                type: 'translate',
                from: `${20} 0`,
                to: '0 0',
                dur: '0.6s',
                begin: `${idx * 0.2}s`,
                fill: 'freeze'
            });

            tagGroup.appendChild(animateOpacity);
            tagGroup.appendChild(animateTransform);

            // 直接添加到容器中，确保在最上层
            container.appendChild(tagGroup);
        });
    },

    // 添加架构标题
    addArchitectureTitle(container) {
        const title = Utils.createSVGElement('text', {
            x: 450,
            y: 50,
            fill: 'url(#layerGradient1)',
            'font-family': 'SF Pro Display, -apple-system, sans-serif',
            'font-size': '24',
            'font-weight': '700',
            'text-anchor': 'middle'
        });
        title.textContent = 'Vivid Architecture';

        const subtitle = Utils.createSVGElement('text', {
            x: 450,
            y: 75,
            fill: 'rgba(255, 255, 255, 0.7)',
            'font-family': 'SF Pro Display, -apple-system, sans-serif',
            'font-size': '14',
            'font-weight': '400',
            'text-anchor': 'middle'
        });
        subtitle.textContent = '分层架构设计，清晰的职责分离';

        container.appendChild(title);
        container.appendChild(subtitle);
    },

    // 初始化安装步骤
    initInstallationSteps() {
        const stepsData = [
            {
                title: '安装 Vivid',
                description: '使用 Go modules 安装 Vivid 框架到您的项目中',
                code: 'go mod init your-project\ngo get github.com/kercylan98/vivid'
            },
            {
                title: '定义 Actor',
                description: '实现 Actor 接口，定义消息处理逻辑',
                code: `type MyActor struct {}\n\nfunc (a *MyActor) OnReceive(ctx vivid.ActorContext) {\n    switch msg := ctx.Message().(type) {\n    case string:\n        fmt.Println("收到消息:", msg)\n    }\n}`
            },
            {
                title: '启动系统',
                description: '创建 Actor 系统，实例化 Actor 并开始消息传递',
                code: `system := vivid.NewActorSystem()\ndefer system.Shutdown()\n\nactor := system.ActorOf(func() vivid.Actor {\n    return &MyActor{}\n})\n\nactor.Tell("Hello World!")`
            }
        ];

        this.renderInstallationSteps(stepsData);
    },

    // 渲染安装步骤
    renderInstallationSteps(stepsData) {
        const container = document.getElementById('installation-steps');
        if (!container) return;

        container.innerHTML = '';

        stepsData.forEach((step, index) => {
            const stepElement = this.createInstallationStep(step, index + 1);
            container.appendChild(stepElement);
        });
    },

    // 创建安装步骤
    createInstallationStep(step, number) {
        const stepDiv = Utils.dom.create('div', {className: 'installation-step'});

        stepDiv.innerHTML = `
      <div class="step-indicator">
        <div class="step-number">${number}</div>
        <div class="step-line"></div>
      </div>
      <div class="step-content">
        <div class="step-header">
          <h3 class="step-title">${step.title}</h3>
          <div class="step-badge">步骤 ${number}</div>
        </div>
        <p class="step-description">${step.description}</p>
        <div class="code-example enhanced">
          <div class="code-header">
            <div class="code-info">
              <span class="code-title">代码示例</span>
              <span class="code-lang">Go</span>
            </div>
            <div class="code-actions">
              <button class="code-action copy-btn" onclick="this.copyCode(this)" data-code="${this.escapeHtml(step.code)}">
                <svg viewBox="0 0 24 24" width="14" height="14">
                  <path d="M8 2v4h8V2h2a2 2 0 012 2v12a2 2 0 01-2 2H6a2 2 0 01-2-2V4a2 2 0 012-2h2z"/>
                  <rect x="6" y="4" width="12" height="2"/>
                </svg>
                复制
              </button>
            </div>
          </div>
          <div class="code-content">
            <pre><code class="language-go">${this.highlightCode(step.code)}</code></pre>
          </div>
          <div class="code-footer">
            <div class="code-stats">
              <span class="stat-item">
                <svg viewBox="0 0 24 24" width="12" height="12">
                  <circle cx="12" cy="12" r="10"/>
                  <polyline points="12,6 12,12 16,14"/>
                </svg>
                即时运行
              </span>
              <span class="stat-item">
                <svg viewBox="0 0 24 24" width="12" height="12">
                  <path d="M9 12l2 2 4-4"/>
                  <path d="M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"/>
                </svg>
                已验证
              </span>
            </div>
          </div>
        </div>
      </div>
    `;

        return stepDiv;
    },

    // 代码高亮
    highlightCode(code) {
        return code
            .replace(/(package|import|func|var|type|defer|switch|case|return)/g, '<span class="keyword">$1</span>')
            .replace(/(vivid\.)/g, '<span class="namespace">$1</span>')
            .replace(/(".*?")/g, '<span class="string">$1</span>')
            .replace(/(\/\/.*)/g, '<span class="comment">$1</span>')
            .replace(/(\d+)/g, '<span class="number">$1</span>');
    },

    // HTML转义
    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    },

    // 复制代码功能
    copyCode(button) {
        const code = button.getAttribute('data-code');
        navigator.clipboard.writeText(code).then(() => {
            const originalText = button.innerHTML;
            button.innerHTML = `
        <svg viewBox="0 0 24 24" width="14" height="14">
          <path d="M9 12l2 2 4-4"/>
          <path d="M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"/>
        </svg>
        已复制
      `;
            button.style.background = 'rgba(40, 202, 66, 0.1)';
            button.style.borderColor = 'rgba(40, 202, 66, 0.3)';
            button.style.color = '#28ca42';

            setTimeout(() => {
                button.innerHTML = originalText;
                button.style.background = '';
                button.style.borderColor = '';
                button.style.color = '';
            }, 2000);
        });
    },

    // 初始化示例展示
    initExamples() {
        const examplesData = [
            {
                title: '聊天系统',
                description: '基于Actor模型的实时消息处理系统，适用于即时通讯应用',
                icon: 'M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z',
                status: 'coming-soon',
                plannedFeatures: ['房间管理', '消息路由', '在线状态']
            },
            {
                title: '任务调度',
                description: '分布式任务调度和处理系统，支持任务的分发与执行',
                icon: 'M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z',
                status: 'coming-soon',
                plannedFeatures: ['任务队列', '负载均衡', '故障恢复']
            },
            {
                title: '流处理',
                description: '实时数据流处理系统，适用于日志分析和事件处理场景',
                icon: 'M11 5.882V19.24a1.76 1.76 0 01-3.417.592l-2.147-6.15M18 13a3 3 0 100-6M5.436 13.683A4.001 4.001 0 017 6h1.832c4.1 0 7.625-1.234 9.168-3v14c-1.543-1.766-5.067-3-9.168-3H7a3.988 3.988 0 01-1.564-.317z',
                status: 'coming-soon',
                plannedFeatures: ['流式处理', '窗口操作', '状态管理']
            }
        ];

        this.renderExamples(examplesData);
    },

    // 渲染示例
    renderExamples(examplesData) {
        const container = document.getElementById('examples-showcase');
        if (!container) return;

        container.innerHTML = '';

        // 创建示例网格
        const grid = Utils.dom.create('div', {className: 'examples-grid'});
        grid.style.display = 'grid';
        grid.style.gridTemplateColumns = 'repeat(auto-fit, minmax(350px, 1fr))';
        grid.style.gap = '2rem';

        examplesData.forEach((example, index) => {
            const card = this.createComingSoonCard(example, index);
            grid.appendChild(card);
        });

        container.appendChild(grid);
    },

    // 创建敬请期待卡片
    createComingSoonCard(example, index) {
        const card = Utils.dom.create('div', {
            className: 'example-card coming-soon',
            'data-aos': 'scale-in',
            'data-aos-delay': index * 150
        });

        card.innerHTML = `
      <div class="example-preview">
        <svg class="example-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor">
          <path d="${example.icon}"/>
        </svg>
        <div class="coming-soon-badge">敬请期待</div>
      </div>
      <div class="example-content">
        <h3 class="example-title">${example.title}</h3>
        <p class="example-description">${example.description}</p>
        <div class="planned-features">
          <h4>计划特性：</h4>
          <ul class="feature-list">
            ${example.plannedFeatures.map(feature => `<li>${feature}</li>`).join('')}
          </ul>
        </div>
        <div class="example-actions">
          <button class="example-link disabled" disabled>开发中...</button>
        </div>
      </div>
    `;

        return card;
    },

    // 初始化粒子效果
    initParticles() {
        const particlesContainer = document.querySelector('.particles');
        if (!particlesContainer) return;

        // 创建粒子
        for (let i = 0; i < 50; i++) {
            const particle = Utils.dom.create('div', {className: 'particle'});

            // 随机位置和动画延迟
            particle.style.left = Utils.random(0, 100) + '%';
            particle.style.animationDelay = Utils.random(0, 10) + 's';
            particle.style.animationDuration = Utils.random(8, 15) + 's';

            particlesContainer.appendChild(particle);
        }
    },

    // 开始动画
    startAnimations() {
        // 统计数字动画
        this.animateStatNumbers();

        // 延迟启动其他动画
        setTimeout(() => {
            this.triggerScrollAnimations();
        }, 500);
    },

    // 统计数字动画
    animateStatNumbers() {
        const statNumbers = document.querySelectorAll('[data-count]');
        statNumbers.forEach(element => {
            const target = parseFloat(element.dataset.count);
            const isDecimal = target % 1 !== 0;

            Utils.animateNumber(element, target, {
                duration: this.config.countUpDuration,
                decimals: isDecimal ? 2 : 0,
                separator: ',',
                easing: 'easeOutCubic'
            });
        });
    },

    // 观察元素进入视口
    observeElements() {
        const observerOptions = {
            threshold: 0.1,
            rootMargin: '0px 0px -50px 0px'
        };

        const observer = new IntersectionObserver((entries) => {
            entries.forEach(entry => {
                if (entry.isIntersecting) {
                    const element = entry.target;
                    Utils.dom.addClass(element, 'animate-slideUp');
                    observer.unobserve(element);
                }
            });
        }, observerOptions);

        // 观察所有可动画元素
        const animatableElements = document.querySelectorAll('[data-aos]');
        animatableElements.forEach(element => {
            observer.observe(element);
        });
    },

    // 触发滚动动画
    triggerScrollAnimations() {
        const animatableElements = document.querySelectorAll('[data-aos]');
        animatableElements.forEach((element, index) => {
            setTimeout(() => {
                if (Utils.isInViewport(element, 0.1)) {
                    Utils.dom.addClass(element, 'animate-slideUp');
                }
            }, index * this.config.animationDelay);
        });
    },

    // 重新计算动画
    recalculateAnimations() {
        // 重新设置动画元素状态
        const animatedElements = document.querySelectorAll('.animate-slideUp');
        animatedElements.forEach(element => {
            if (!Utils.isInViewport(element, 0.1)) {
                Utils.dom.removeClass(element, 'animate-slideUp');
            }
        });
    }
};

// 页面加载完成后初始化应用
document.addEventListener('DOMContentLoaded', () => {
    VividApp.init();

    // 添加全局复制函数
    window.copyCode = function (button) {
        VividApp.copyCode(button);
    };
});

// 页面完全加载后的处理
window.addEventListener('load', () => {
    // 移除加载状态
    Utils.dom.removeClass(document.body, 'loading');

    // 开始背景动画
    VividApp.startBackgroundAnimations();
});

// 导出到全局作用域以便调试
window.VividApp = VividApp; 