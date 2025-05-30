// 动画效果管理器
'use strict';

const AnimationManager = {
    // 动画状态
    state: {
        isAnimating: false,
        particleCount: 0,
        backgroundEffects: {
            neuralNetwork: null,
            particles: [],
            gradientOrbs: null
        }
    },

    // 配置
    config: {
        particleCount: 100,
        neuralNetworkNodes: 20,
        gradientOrbCount: 3,
        animationSpeed: 1,
        enablePerformanceMode: false
    },

    // 初始化动画系统
    init() {
        this.createBackgroundEffects();
        this.initParticleSystem();
        this.initNeuralNetwork();
        this.initGradientOrbs();
        this.bindPerformanceOptimizations();
        this.startAnimationLoop();
    },

    // 创建背景效果
    createBackgroundEffects() {
        const backgroundContainer = document.querySelector('.background-effects');
        if (!backgroundContainer) return;

        // 确保容器为空
        backgroundContainer.innerHTML = '';

        // 创建神经网络容器
        const neuralNetwork = Utils.dom.create('div', {className: 'neural-network'});
        backgroundContainer.appendChild(neuralNetwork);

        // 创建粒子容器
        const particles = Utils.dom.create('div', {className: 'particles'});
        backgroundContainer.appendChild(particles);

        // 创建渐变球体容器
        const gradientOrbs = Utils.dom.create('div', {className: 'gradient-orbs'});
        backgroundContainer.appendChild(gradientOrbs);

        this.state.backgroundEffects.neuralNetwork = neuralNetwork;
        this.state.backgroundEffects.particles = particles;
        this.state.backgroundEffects.gradientOrbs = gradientOrbs;
    },

    // 初始化粒子系统
    initParticleSystem() {
        const container = this.state.backgroundEffects.particles;
        if (!container) return;

        // 创建粒子
        for (let i = 0; i < this.config.particleCount; i++) {
            const particle = this.createParticle(i);
            container.appendChild(particle);
            this.state.backgroundEffects.particles.push(particle);
        }

        this.state.particleCount = this.config.particleCount;
    },

    // 创建单个粒子
    createParticle(index) {
        const particle = Utils.dom.create('div', {className: 'particle'});

        // 随机粒子属性
        const size = Utils.random(1, 4);
        const x = Utils.random(0, 100);
        const y = Utils.random(0, 100);
        const duration = Utils.random(10, 30);
        const delay = Utils.random(0, 20);

        // 颜色变体
        const colors = ['#00d4ff', '#6c5ce7', '#a29bfe', '#fd79a8'];
        const color = colors[index % colors.length];

        particle.style.cssText = `
      position: absolute;
      left: ${x}%;
      top: ${y}%;
      width: ${size}px;
      height: ${size}px;
      background: ${color};
      border-radius: 50%;
      opacity: ${Utils.random(0.3, 0.8)};
      animation: particleFloat ${duration}s ${delay}s linear infinite;
      pointer-events: none;
      filter: blur(${size > 2 ? 1 : 0}px);
    `;

        return particle;
    },

    // 初始化神经网络效果
    initNeuralNetwork() {
        const container = this.state.backgroundEffects.neuralNetwork;
        if (!container) return;

        // 创建SVG画布
        const svg = Utils.createSVGElement('svg', {
            width: '100%',
            height: '100%',
            style: 'position: absolute; top: 0; left: 0; pointer-events: none;'
        });

        container.appendChild(svg);

        // 创建网络节点
        const nodes = this.createNetworkNodes();
        const connections = this.createNetworkConnections(nodes);

        // 添加节点到SVG
        nodes.forEach(node => svg.appendChild(node.element));
        connections.forEach(connection => svg.appendChild(connection));

        // 启动网络动画
        this.animateNeuralNetwork(nodes, connections);
    },

    // 创建网络节点
    createNetworkNodes() {
        const nodes = [];
        const nodeCount = this.config.neuralNetworkNodes;

        for (let i = 0; i < nodeCount; i++) {
            const x = Utils.random(10, 90);
            const y = Utils.random(10, 90);
            const radius = Utils.random(2, 6);

            const circle = Utils.createSVGElement('circle', {
                cx: `${x}%`,
                cy: `${y}%`,
                r: radius,
                fill: '#00d4ff',
                opacity: Utils.random(0.3, 0.7),
                class: 'network-node'
            });

            nodes.push({
                element: circle,
                x: x,
                y: y,
                radius: radius,
                connections: []
            });
        }

        return nodes;
    },

    // 创建网络连接
    createNetworkConnections(nodes) {
        const connections = [];

        nodes.forEach((node, index) => {
            // 为每个节点创建2-4个连接
            const connectionCount = Utils.random(2, 4, true);

            for (let i = 0; i < connectionCount; i++) {
                const targetIndex = Utils.random(0, nodes.length - 1, true);
                if (targetIndex === index) continue;

                const target = nodes[targetIndex];
                const distance = Math.sqrt(
                    Math.pow(node.x - target.x, 2) + Math.pow(node.y - target.y, 2)
                );

                // 只连接相对较近的节点
                if (distance < 30) {
                    const line = Utils.createSVGElement('line', {
                        x1: `${node.x}%`,
                        y1: `${node.y}%`,
                        x2: `${target.x}%`,
                        y2: `${target.y}%`,
                        stroke: '#6c5ce7',
                        'stroke-width': 1,
                        opacity: 0.2,
                        class: 'network-connection'
                    });

                    connections.push(line);
                    node.connections.push(target);
                }
            }
        });

        return connections;
    },

    // 动画神经网络
    animateNeuralNetwork(nodes, connections) {
        let animationIndex = 0;

        const pulseNetwork = () => {
            // 随机选择一个节点开始脉冲
            const startNode = nodes[Utils.random(0, nodes.length - 1, true)];

            // 脉冲动画
            this.pulseNode(startNode.element);

            // 延迟传播到连接的节点
            setTimeout(() => {
                startNode.connections.forEach((connectedNode, index) => {
                    setTimeout(() => {
                        this.pulseNode(connectedNode.element);
                    }, index * 200);
                });
            }, 300);

            // 下一次脉冲
            setTimeout(pulseNetwork, Utils.random(3000, 8000));
        };

        // 启动脉冲动画
        setTimeout(pulseNetwork, 2000);
    },

    // 节点脉冲动画
    pulseNode(nodeElement) {
        const originalRadius = parseFloat(nodeElement.getAttribute('r'));
        const originalOpacity = parseFloat(nodeElement.getAttribute('opacity'));

        // 脉冲效果
        nodeElement.style.animation = 'none';
        nodeElement.style.transform = 'scale(1)';

        const keyframes = [
            {transform: 'scale(1)', opacity: originalOpacity},
            {transform: 'scale(2)', opacity: originalOpacity * 1.5},
            {transform: 'scale(1)', opacity: originalOpacity}
        ];

        nodeElement.animate(keyframes, {
            duration: 600,
            easing: 'ease-out'
        });
    },

    // 初始化渐变球体
    initGradientOrbs() {
        const container = this.state.backgroundEffects.gradientOrbs;
        if (!container) return;

        for (let i = 0; i < this.config.gradientOrbCount; i++) {
            const orb = this.createGradientOrb(i);
            container.appendChild(orb);
        }
    },

    // 创建渐变球体
    createGradientOrb(index) {
        const orb = Utils.dom.create('div', {className: 'gradient-orb'});

        const size = Utils.random(200, 400);
        const x = Utils.random(0, 100);
        const y = Utils.random(0, 100);
        const duration = Utils.random(20, 40);
        const delay = index * 5;

        const gradients = [
            'radial-gradient(circle, rgba(0, 212, 255, 0.15) 0%, transparent 70%)',
            'radial-gradient(circle, rgba(108, 92, 231, 0.12) 0%, transparent 70%)',
            'radial-gradient(circle, rgba(162, 155, 254, 0.1) 0%, transparent 70%)'
        ];

        orb.style.cssText = `
      position: absolute;
      left: ${x}%;
      top: ${y}%;
      width: ${size}px;
      height: ${size}px;
      background: ${gradients[index % gradients.length]};
      border-radius: 50%;
      animation: float ${duration}s ${delay}s ease-in-out infinite;
      pointer-events: none;
      filter: blur(2px);
    `;

        return orb;
    },

    // 启动动画循环
    startAnimationLoop() {
        this.state.isAnimating = true;
        this.animationLoop();
    },

    // 主动画循环
    animationLoop() {
        if (!this.state.isAnimating) return;

        // 更新粒子位置（如果需要）
        this.updateParticles();

        // 性能检查
        if (this.config.enablePerformanceMode) {
            this.optimizePerformance();
        }

        requestAnimationFrame(() => this.animationLoop());
    },

    // 更新粒子
    updateParticles() {
        // 基于鼠标位置的交互效果
        const mouseX = this.mousePosition ? this.mousePosition.x : 0;
        const mouseY = this.mousePosition ? this.mousePosition.y : 0;

        if (mouseX > 0 && mouseY > 0) {
            this.applyMouseInteraction(mouseX, mouseY);
        }
    },

    // 鼠标交互效果
    applyMouseInteraction(mouseX, mouseY) {
        const particles = document.querySelectorAll('.particle');
        const maxDistance = 100;

        particles.forEach(particle => {
            const rect = particle.getBoundingClientRect();
            const particleX = rect.left + rect.width / 2;
            const particleY = rect.top + rect.height / 2;

            const distance = Math.sqrt(
                Math.pow(mouseX - particleX, 2) + Math.pow(mouseY - particleY, 2)
            );

            if (distance < maxDistance) {
                const force = (maxDistance - distance) / maxDistance;
                const angle = Math.atan2(particleY - mouseY, particleX - mouseX);

                const offsetX = Math.cos(angle) * force * 20;
                const offsetY = Math.sin(angle) * force * 20;

                particle.style.transform = `translate(${offsetX}px, ${offsetY}px)`;
                particle.style.opacity = Math.min(1, parseFloat(particle.style.opacity) + force * 0.3);
            } else {
                particle.style.transform = 'translate(0, 0)';
            }
        });
    },

    // 性能优化
    optimizePerformance() {
        const fps = this.calculateFPS();

        if (fps < 30) {
            // 降低粒子数量
            this.reduceParticleCount();

            // 简化动画
            this.simplifyAnimations();
        }
    },

    // 计算FPS
    calculateFPS() {
        if (!this.lastFrameTime) {
            this.lastFrameTime = performance.now();
            this.frameCount = 0;
            return 60;
        }

        this.frameCount++;
        const currentTime = performance.now();
        const timeDiff = currentTime - this.lastFrameTime;

        if (timeDiff >= 1000) {
            const fps = (this.frameCount / timeDiff) * 1000;
            this.frameCount = 0;
            this.lastFrameTime = currentTime;
            return fps;
        }

        return 60;
    },

    // 减少粒子数量
    reduceParticleCount() {
        const particles = document.querySelectorAll('.particle');
        const removeCount = Math.floor(particles.length * 0.2);

        for (let i = 0; i < removeCount; i++) {
            if (particles[i] && particles[i].parentNode) {
                particles[i].parentNode.removeChild(particles[i]);
            }
        }

        this.state.particleCount = particles.length - removeCount;
    },

    // 简化动画
    simplifyAnimations() {
        // 禁用复杂的滤镜效果
        const particles = document.querySelectorAll('.particle');
        particles.forEach(particle => {
            particle.style.filter = 'none';
        });

        // 减少动画复杂度
        this.config.enablePerformanceMode = true;
    },

    // 绑定性能优化
    bindPerformanceOptimizations() {
        // 鼠标移动事件（节流）
        Utils.events.on(document, 'mousemove', Utils.throttle((event) => {
            this.mousePosition = {x: event.clientX, y: event.clientY};
        }, 16));

        // 窗口失焦时暂停动画
        Utils.events.on(window, 'blur', () => {
            this.pauseAnimations();
        });

        Utils.events.on(window, 'focus', () => {
            this.resumeAnimations();
        });

        // 页面可见性变化
        if (typeof document.hidden !== 'undefined') {
            Utils.events.on(document, 'visibilitychange', () => {
                if (document.hidden) {
                    this.pauseAnimations();
                } else {
                    this.resumeAnimations();
                }
            });
        }
    },

    // 暂停动画
    pauseAnimations() {
        this.state.isAnimating = false;

        // 暂停CSS动画
        const animatedElements = document.querySelectorAll('.particle, .gradient-orb, .network-connection');
        animatedElements.forEach(element => {
            element.style.animationPlayState = 'paused';
        });
    },

    // 恢复动画
    resumeAnimations() {
        this.state.isAnimating = true;

        // 恢复CSS动画
        const animatedElements = document.querySelectorAll('.particle, .gradient-orb, .network-connection');
        animatedElements.forEach(element => {
            element.style.animationPlayState = 'running';
        });

        this.animationLoop();
    },

    // 创建文本动画效果
    createTextAnimation(element, text, options = {}) {
        const {
            speed = 50,
            cursor = true,
            loop = false,
            delay = 0
        } = options;

        return new Promise((resolve) => {
            setTimeout(() => {
                let index = 0;
                element.textContent = '';

                const type = () => {
                    if (index < text.length) {
                        element.textContent += text.charAt(index);
                        index++;
                        setTimeout(type, speed);
                    } else {
                        if (cursor) {
                            element.style.borderRight = '2px solid #00d4ff';
                            element.style.animation = 'blink 1s infinite';
                        }

                        if (loop) {
                            setTimeout(() => {
                                element.textContent = '';
                                index = 0;
                                type();
                            }, 2000);
                        } else {
                            resolve();
                        }
                    }
                };

                type();
            }, delay);
        });
    },

    // 创建滚动触发动画
    createScrollAnimation(elements, animationClass = 'animate-slideUp') {
        const observer = new IntersectionObserver((entries) => {
            entries.forEach(entry => {
                if (entry.isIntersecting) {
                    Utils.dom.addClass(entry.target, animationClass);
                    observer.unobserve(entry.target);
                }
            });
        }, {
            threshold: 0.1,
            rootMargin: '0px 0px -100px 0px'
        });

        elements.forEach(element => {
            observer.observe(element);
        });

        return observer;
    },

    // 创建数字计数动画
    createCountUpAnimation(element, target, options = {}) {
        const {
            duration = 2000,
            decimals = 0,
            separator = ',',
            prefix = '',
            suffix = '',
            easing = 'easeOutCubic'
        } = options;

        Utils.animateNumber(element, target, {
            duration,
            decimals,
            separator,
            prefix,
            suffix,
            easing
        });
    },

    // 销毁动画系统
    destroy() {
        this.state.isAnimating = false;

        // 清理事件监听器
        Utils.events.off(document, 'mousemove');
        Utils.events.off(window, 'blur');
        Utils.events.off(window, 'focus');
        Utils.events.off(document, 'visibilitychange');

        // 清理DOM元素
        const backgroundContainer = document.querySelector('.background-effects');
        if (backgroundContainer) {
            backgroundContainer.innerHTML = '';
        }

        // 重置状态
        this.state = {
            isAnimating: false,
            particleCount: 0,
            backgroundEffects: {
                neuralNetwork: null,
                particles: [],
                gradientOrbs: null
            }
        };
    }
};

// 页面加载后自动初始化动画系统
document.addEventListener('DOMContentLoaded', () => {
    AnimationManager.init();
});

// 导出到全局作用域
window.AnimationManager = AnimationManager;

// 为VividApp添加背景动画启动方法
if (window.VividApp) {
    window.VividApp.startBackgroundAnimations = () => {
        AnimationManager.resumeAnimations();
    };
} 