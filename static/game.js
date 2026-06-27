const canvas=document.getElementById('game');
const ctx=canvas.getContext('2d');
const info=document.getElementById('info');
const dashFill=document.getElementById('dashFill');
const orbInfo=document.getElementById('orbInfo');
const chatMessagesEl=document.getElementById('chat-messages');
const chatInput=document.getElementById('chat-input');
const chatSend=document.getElementById('chat-send');
const SCREEN_W=800,SCREEN_H=600;


const ws=new WebSocket('ws://'+location.host+'/ws');
let myId=null,players={},platforms=[],orbs=[],thrownOrbs=[],explosions=[],chatMessages=[],bots=[];
let worldWidth=2400,worldHeight=600;
const keys={left:false,right:false,jump:false,dash:false,throwOrb:false,detonate:false};
const SIZE=30,ORB_SIZE=14,EXPLOSION_WIDTH=45;
let cameraX=0,cameraY=0,shakeAmount=0,pendingChat="";
let prevHasOrb = false;
let prevRespawnTimer = 0;
let deathFlash = 0;

// ─── Sound Manager ───
var sounds = {};
var audioUnlocked = false;
var prevOnGround = true;
var prevLastDash = 0;
var prevHasOrbThrow = false;
var prevBots = [];

function loadSound(name, file) {
    var audio = new Audio('sounds/' + file);
    audio.preload = 'auto';
    audio.volume = 0.4;
    sounds[name] = audio;
}

function playSound(name) {
    if (!audioUnlocked) return; // Block ALL sounds until first click
    var s = sounds[name];
    if (s) {
        s.currentTime = 0;
        s.play().catch(function() {});
    }
}

// Load sounds
loadSound('jump', 'jump.mp3');
loadSound('dash', 'dash.mp3');
loadSound('orbPickup', 'orb_pickup.mp3');
loadSound('orbThrow', 'orb_throw.mp3');
loadSound('explosion', 'explosion.mp3');
loadSound('death', 'death.mp3');
loadSound('respawn', 'respawn.mp3');

// Unlock audio on first interaction
document.getElementById('start-overlay').addEventListener('click', function() {
    this.style.display = 'none';
    audioUnlocked = true;
    // Unlock all sounds
    for (var key in sounds) {
        var s = sounds[key];
        s.play().then(function() { s.pause(); s.currentTime = 0; }).catch(function() {});
    }
});

// Click-to-start overlay
document.getElementById('start-overlay').addEventListener('click', function() {
    this.style.display = 'none';
    // Unlock all sounds
    for (var key in sounds) {
        sounds[key].play().then(function(s) { s.pause(); s.currentTime = 0; }).catch(function() {});
    }
});


// Input
document.addEventListener('keydown',e=>{
    if(document.activeElement===chatInput){
        if(e.key==='Enter'){sendChat();e.preventDefault();}
        return;
    }
    if(e.key==='ArrowLeft'||e.key==='a')keys.left=true;
    if(e.key==='ArrowRight'||e.key==='d')keys.right=true;
    if(e.key==='ArrowUp'||e.key==='w'||e.key===' '){keys.jump=true;e.preventDefault();}
    if(e.key==='Shift'){keys.dash=true;e.preventDefault();}
    if(e.key==='e'||e.key==='E'){keys.throwOrb=true;e.preventDefault();}
    if(e.key==='q'||e.key==='Q'){keys.detonate=true;e.preventDefault();}
    if(e.key==='t'||e.key==='T'){chatInput.focus();e.preventDefault();}
});
document.addEventListener('keyup',e=>{
    if(e.key==='ArrowLeft'||e.key==='a')keys.left=false;
    if(e.key==='ArrowRight'||e.key==='d')keys.right=false;
    if(e.key==='ArrowUp'||e.key==='w'||e.key===' ')keys.jump=false;
    if(e.key==='Shift')keys.dash=false;
    if(e.key==='e'||e.key==='E')keys.throwOrb=false;
    if(e.key==='q'||e.key==='Q')keys.detonate=false;
});

// ─── Mobile Controls ───
function setupMobileButton(id, key) {
    var btn = document.getElementById(id);
    if (!btn) return;
    
    btn.addEventListener('touchstart', function(e) {
        e.preventDefault();
        keys[key] = true;
        btn.style.transform = 'scale(0.93)';
    });
    btn.addEventListener('touchend', function(e) {
        e.preventDefault();
        keys[key] = false;
        btn.style.transform = 'scale(1)';
    });
    btn.addEventListener('touchcancel', function(e) {
        keys[key] = false;
        btn.style.transform = 'scale(1)';
    });
    // Mouse fallback for testing on desktop
    btn.addEventListener('mousedown', function(e) {
        e.preventDefault();
        keys[key] = true;
    });
    btn.addEventListener('mouseup', function(e) {
        e.preventDefault();
        keys[key] = false;
    });
    btn.addEventListener('mouseleave', function(e) {
        keys[key] = false;
    });
}

setupMobileButton('ctrl-left', 'left');
setupMobileButton('ctrl-right', 'right');
setupMobileButton('ctrl-jump', 'jump');
setupMobileButton('ctrl-dash', 'dash');
setupMobileButton('ctrl-throw', 'throwOrb');


// Chat
function sendChat(){
    const text=chatInput.value.trim();
    if(text){pendingChat=text;chatInput.value='';}
}
chatSend.addEventListener('click',sendChat);
chatInput.addEventListener('keydown',e=>{
    if(e.key==='Enter'){sendChat();e.preventDefault();}
});

function addChatMessage(msg){
    const div=document.createElement('div');
    div.className='chat-msg'+(msg.playerId===0?' system':'');
    if(msg.playerId===0){
        div.textContent=msg.text;
    }else{
        const nameSpan=document.createElement('span');
        nameSpan.className='name';
        nameSpan.textContent='P'+msg.playerId+': ';
        nameSpan.style.color=getPlayerColorCSS(msg.color);
        const textSpan=document.createElement('span');
        textSpan.className='text';
        textSpan.textContent=msg.text;
        div.appendChild(nameSpan);
        div.appendChild(textSpan);
    }
    chatMessagesEl.appendChild(div);
    chatMessagesEl.scrollTop=chatMessagesEl.scrollHeight;
}

function getPlayerColorCSS(color){
    const map={blue:'#4488ff',red:'#ff4444',green:'#44cc44',orange:'#ff8844',
        purple:'#cc44cc',yellow:'#cccc44',cyan:'#44cccc',pink:'#ff88cc'};
    return map[color]||'#fff';
}

// Network
ws.onopen=()=>info.textContent='Connected!';
ws.onclose=()=>info.textContent='Disconnected';
ws.onmessage=e=>{
    const m=JSON.parse(e.data);
    if(m.type==='init'){
        myId=m.playerId;players=m.players;platforms=m.platforms||[];
        orbs=m.orbs||[];thrownOrbs=m.thrownOrbs||[];explosions=m.explosions||[];
        bots=m.bots||[];
        chatMessages=m.chatMessages||[];
        worldWidth=m.worldWidth||2400;worldHeight=m.worldHeight||600;
        chatMessagesEl.innerHTML='';
        for(const msg of chatMessages)addChatMessage(msg);
    }else if(m.type==='playerJoined'){
        players[m.playerId]=m.player;
    }else if(m.type==='gameState'){
        const oldExplosions=(explosions||[]).length;
        players=m.players;orbs=m.orbs||[];thrownOrbs=m.thrownOrbs||[];
        explosions=m.explosions||[];
        bots=m.bots||[];
        const newChat=m.chatMessages||[];
        if(newChat.length>chatMessages.length){
            for(let i=chatMessages.length;i<newChat.length;i++)addChatMessage(newChat[i]);
        }
        chatMessages=newChat;
        if(explosions.length>oldExplosions){
            shakeAmount=8;
            playSound('explosion');
        }
        updateUI();
        // Bot sound detection
        if (prevBots.length === bots.length) {
            for (var bi = 0; bi < bots.length; bi++) {
                var bot = bots[bi];
                var prev = prevBots[bi];
                if (prev && bot) {
                    // Bot jumped (was on ground, now not, negative Vy)
                    if (prev.onGround && !bot.onGround && (bot.vy || 0) < -2) {
                        playSound('jump');
                    }
                    // Bot dashed
                    if ((prev.lastDash || 0) !== (bot.lastDash || 0) && bot.lastDash > 0) {
                        console.log('Bot dashed', bot.id, bot.lastDash);
                        playSound('dash');
                    }
                    // Bot picked up orb
                    if (!prev.hasOrb && bot.hasOrb && !bot.respawnTimer) {
                        playSound('orbPickup');
                    }
                    // Bot threw orb (had orb, now doesn't)
                    if (prev.hasOrb && !bot.hasOrb && !bot.respawnTimer) {
                        playSound('orbThrow');
                    }
                    // Bot died (was alive, now respawning)
                    if (!prev.respawnTimer && bot.respawnTimer > 0) {
                        playSound('death');
                        deathFlash = 1.5;
                    }
                    // Bot respawned
                    if (prev.respawnTimer > 0 && !bot.respawnTimer) {
                        playSound('respawn');
                    }
                }
            }
        }
        // Deep copy bots for next comparison
        prevBots = bots.map(function(b) {
            return {
                onGround: b.onGround,
                hasOrb: b.hasOrb,
                vy: b.vy,
                respawnTimer: b.respawnTimer,
                lastDash: b.lastDash || 0
            };
        });
    }else if(m.type==='playerLeft'){
        delete players[m.playerId];
    }
};

function updateUI(){
    var me = players[myId];
    if (!me) return;

    // Orb pickup detection
    if (me.hasOrb && !prevHasOrb) {
        playSound('orbPickup');
    }

    
    // Death detection
    if (me.respawnTimer > 0 && prevRespawnTimer === 0) {
        playSound('death');
        deathFlash = 1.5;
    }

    if (me.respawnTimer > 0 && deathFlash <= 0) {
        deathFlash = 0.1;
    }

    if (me.respawnTimer === 0 && prevRespawnTimer > 0) {
        playSound('respawn');
    }
    // Jump detection (player was on ground, now in air with negative Vy)
    if (prevOnGround && !me.onGround && (me.vy || 0) < -2) {
        playSound('jump');
    }
    // Dash detection
    if (me.lastDash && me.lastDash !== prevLastDash) {
        playSound('dash');
    }

    // Orb throw detection (had orb, now doesn't, and there's a new thrownOrb)
    if (prevHasOrb && !me.hasOrb && !me.respawnTimer) {
        playSound('orbThrow');
    }
    prevOnGround = me.onGround;
    prevLastDash = me.lastDash || 0;
    prevHasOrb = me.hasOrb;
    prevRespawnTimer = me.respawnTimer || 0;

    var now = Date.now() / 1000;
    var elapsed = now - (me.lastDash || 0);
    var cd = me.dashCooldown || 0.3;
    var pct = Math.min(100, (elapsed / cd) * 100);
    dashFill.style.width = pct + '%';
    dashFill.style.background = pct >= 100 ? '#ffaa00' : '#666';
    if (me.hasOrb) {
        orbInfo.textContent = '⚡ ORB HELD - Press E';
    } else if (thrownOrbs && thrownOrbs.length > 0) {
        var myOrb = null;
        for (var i = 0; i < thrownOrbs.length; i++) {
            if (thrownOrbs[i].ownerId === myId) {
                myOrb = thrownOrbs[i];
                break;
            }
        }
        if (myOrb) {
            var remaining = Math.max(0, (myOrb.fuseEnd - now)).toFixed(1);
            orbInfo.textContent = '⚡ ' + remaining + 's - Q=detonate';
        } else {
            orbInfo.textContent = '';
        }
    } else {
        orbInfo.textContent = '';
    }

}

// Send input (with chat)
setInterval(()=>{
    if(ws.readyState!==1)return;
    const payload={...keys};
    if(pendingChat){payload.chatMsg=pendingChat;pendingChat="";}
    ws.send(JSON.stringify(payload));
},1000/60);

// ─── Drawing ───
function drawOrb(x, y, time, orbType) {
    orbType = orbType || 'down';
    var cx = x + ORB_SIZE / 2, cy = y + ORB_SIZE / 2;
    var pulse = 1 + Math.sin(time * 4) * 0.2;
    var glowSize = ORB_SIZE * pulse * 1.5;
    var r1, g1, b1, r2, g2, b2;
    if (orbType === 'up') {
        // Green/purple for roof orb
        r1 = 180; g1 = 120; b1 = 220;
        r2 = 140; g2 = 80;  b2 = 180;
    } else {
        // Blue/cyan for floor orb (original)
        r1 = 100; g1 = 200; b1 = 255;
        r2 = 0; g2 = 100; b2 = 255;
    }
    var grad = ctx.createRadialGradient(cx, cy, 2, cx, cy, glowSize);
    grad.addColorStop(0, 'rgba(' + r1 + ',' + g1 + ',' + b1 + ',0.8)');
    grad.addColorStop(0.5, 'rgba(' + r2 + ',' + g2 + ',' + b2 + ',0.3)');
    grad.addColorStop(1, 'rgba(0,50,255,0)');
    ctx.fillStyle = grad;
    ctx.beginPath(); ctx.arc(cx, cy, glowSize, 0, Math.PI * 2); ctx.fill();
    ctx.fillStyle = orbType === 'up' ? '#9966cc' : '#0066cc';
    ctx.fillRect(x, y, ORB_SIZE, ORB_SIZE);
    ctx.strokeStyle = orbType === 'up' ? '#cc99ff' : '#00ffff';
    ctx.lineWidth = 2; ctx.strokeRect(x, y, ORB_SIZE, ORB_SIZE);
    ctx.strokeStyle = '#000'; ctx.lineWidth = 1;
    ctx.strokeRect(x - 1, y - 1, ORB_SIZE + 2, ORB_SIZE + 2);
    ctx.fillStyle = '#ffff00'; ctx.font = '10px Arial'; ctx.textAlign = 'center';
    ctx.fillText(orbType === 'up' ? '⬆' : '⚡', cx, cy + 4);
}

function drawThrownOrb(to, time) {
    if (!to || to.x === undefined || to.y === undefined) return;
    var cx = to.x + ORB_SIZE / 2;
    var cy = to.y + ORB_SIZE / 2;
    var fuseLeft = Math.max(0, (to.fuseEnd || 0) - time);
    var isUp = (to.orbType === 'up');
    var glowSize = Math.max(2, ORB_SIZE * 2 * (1 + (1 - fuseLeft / 3)));

    // Glow gradient
    var grad = ctx.createRadialGradient(cx, cy, 2, cx, cy, glowSize);
    if (isUp) {
        grad.addColorStop(0, 'rgba(180,120,220,0.8)');
        grad.addColorStop(0.4, 'rgba(150,80,180,0.4)');
        grad.addColorStop(1, 'rgba(100,50,150,0)');
    } else {
        var r = fuseLeft < 0.5 ? 255 : 100;
        grad.addColorStop(0, 'rgba(' + r + ',200,255,0.9)');
        grad.addColorStop(0.4, 'rgba(255,150,50,0.5)');
        grad.addColorStop(1, 'rgba(255,50,0,0)');
    }
    ctx.fillStyle = grad;
    ctx.beginPath();
    ctx.arc(cx, cy, glowSize, 0, Math.PI * 2);
    ctx.fill();

    // Body (flashing)
    var flash = fuseLeft < 1 ? Math.sin(time * 30) * 0.5 + 0.5 : 0;
    if (isUp) {
        ctx.fillStyle = 'rgb(' + Math.floor(140 + flash * 80) + ',0,' + Math.floor(160 + flash * 60) + ')';
    } else {
        ctx.fillStyle = 'rgb(' + Math.floor(flash * 255) + ',' + Math.floor(100 * (1 - flash)) + ',' + Math.floor(200 * (1 - flash)) + ')';
    }
    ctx.fillRect(to.x, to.y, ORB_SIZE, ORB_SIZE);

    // Border
    var bR = Math.floor(255 * (1 - fuseLeft / 3));
    if (isUp) {
        ctx.strokeStyle = 'rgb(200,' + (180 - bR) + ',255)';
    } else {
        ctx.strokeStyle = 'rgb(' + bR + ',' + (255 - bR) + ',255)';
    }
    ctx.lineWidth = 2;
    ctx.strokeRect(to.x, to.y, ORB_SIZE, ORB_SIZE);
    ctx.strokeStyle = '#000';
    ctx.lineWidth = 1;
    ctx.strokeRect(to.x - 1, to.y - 1, ORB_SIZE + 2, ORB_SIZE + 2);

    // Icon
    ctx.fillStyle = '#ffff00';
    ctx.font = '10px Arial';
    ctx.textAlign = 'center';
    ctx.fillText(isUp ? '⬆' : '⚡', cx, cy + 4);
}

function drawPlayer(p, id, time) {
    if (!p || p.x === undefined) return;
        if (p.respawnTimer > 0) {
            if (Math.floor(time * 10) % 2 === 0) return; // Flash
            ctx.globalAlpha = 0.4;
            // Show respawn countdown
            ctx.fillStyle = '#fff';
            ctx.font = '11px Arial';
            ctx.textAlign = 'center';
            ctx.fillText('Respawning... ' + p.respawnTimer.toFixed(1), p.x + SIZE/2, p.y - 14);
        }
    ctx.fillStyle = 'rgba(0,0,0,0.3)';
    ctx.fillRect(p.x + 3, p.y + 3, SIZE, SIZE);
    ctx.fillStyle = p.color || '#fff';
    ctx.fillRect(p.x, p.y, SIZE, SIZE);
    if (p.hasOrb) {
        var pulse = 1 + Math.sin(time * 6) * 0.15;
        ctx.fillStyle = 'rgba(0,200,255,' + (0.3 * pulse) + ')';
        ctx.fillRect(p.x - 4, p.y - 4, SIZE + 8, SIZE + 8);
        ctx.strokeStyle = 'rgba(0,255,255,' + (0.7 * pulse) + ')';
        ctx.lineWidth = 1.5;
        ctx.strokeRect(p.x - 4, p.y - 4, SIZE + 8, SIZE + 8);
    }
    ctx.fillStyle = '#fff';
    var eyeY = p.y + 8;
    var facing = p.facing || 1;
    ctx.fillRect(p.x + 6, eyeY, 6, 6);
    ctx.fillRect(p.x + 18, eyeY, 6, 6);
    ctx.fillStyle = '#111';
    if (facing > 0) {
        ctx.fillRect(p.x + 9, eyeY + 1, 4, 4);
        ctx.fillRect(p.x + 21, eyeY + 1, 4, 4);
    } else {
        ctx.fillRect(p.x + 5, eyeY + 1, 4, 4);
        ctx.fillRect(p.x + 17, eyeY + 1, 4, 4);
    }
    if (p.lastDash && time - p.lastDash < 0.1) {
        ctx.fillStyle = '#ffaa00';
        ctx.fillRect(p.x - 2, p.y + SIZE / 2 - 4, 4, 8);
        ctx.fillRect(p.x + SIZE - 2, p.y + SIZE / 2 - 4, 4, 8);
    }
    ctx.fillStyle = '#fff';
    ctx.font = 'bold 12px Arial';
    ctx.textAlign = 'center';
    ctx.fillText('P' + id, p.x + SIZE / 2, p.y - 8);
    ctx.globalAlpha = 1;
}


function draw() {
    try {
        ctx.clearRect(0, 0, SCREEN_W, SCREEN_H);
                // Death flash effect
        if (deathFlash > 0) {
            ctx.save();
            ctx.resetTransform();
            var alpha = deathFlash / 1.5;
            ctx.fillStyle = 'rgba(255,255,255,' + (alpha * 0.7) + ')';
            ctx.fillRect(0, 0, SCREEN_W, SCREEN_H);
            // Vignette effect
            var gradient = ctx.createRadialGradient(SCREEN_W/2, SCREEN_H/2, SCREEN_W*0.3, SCREEN_W/2, SCREEN_H/2, SCREEN_W*0.8);
            gradient.addColorStop(0, 'rgba(0,0,0,0)');
            gradient.addColorStop(1, 'rgba(0,0,0,' + (alpha * 0.8) + ')');
            ctx.fillStyle = gradient;
            ctx.fillRect(0, 0, SCREEN_W, SCREEN_H);
            ctx.restore();
            deathFlash -= 0.016;
            if (deathFlash < 0) deathFlash = 0;
        }
        var time = Date.now() / 1000;
        var me = players[myId];
        if (!me) { requestAnimationFrame(draw); return; }
        cameraX = me.x + SIZE / 2 - SCREEN_W / 2;
        cameraY = me.y + SIZE / 2 - SCREEN_H / 2;
        cameraX = Math.max(0, Math.min(worldWidth - SCREEN_W, cameraX));
        cameraY = Math.max(0, Math.min(worldHeight - SCREEN_H, cameraY));
        if (shakeAmount > 0.1) {
            cameraX += (Math.random() - 0.5) * shakeAmount * 2;
            cameraY += (Math.random() - 0.5) * shakeAmount * 2;
            shakeAmount *= 0.85;
        } else {
            shakeAmount = 0;
        }
        ctx.save();
        ctx.translate(-cameraX, -cameraY);
        ctx.strokeStyle = '#ff000033';
        ctx.lineWidth = 3;
        ctx.strokeRect(0, 0, worldWidth, worldHeight);
        ctx.strokeStyle = '#1a2a1a';
        ctx.lineWidth = 1;
        for (var i = 0; i < worldWidth; i += 100) { ctx.beginPath(); ctx.moveTo(i, 0); ctx.lineTo(i, worldHeight); ctx.stroke(); }
        for (var j = 0; j < worldHeight; j += 100) { ctx.beginPath(); ctx.moveTo(0, j); ctx.lineTo(worldWidth, j); ctx.stroke(); }
        // Pulsing platform color (green to blue every 2 seconds)
        var colorOsc = Math.sin(time * Math.PI/15) * 0.5 + 0.5; // 0 to 1, 2-second cycle
        var r = Math.floor(30 + colorOsc * 20);
        var g = Math.floor(100 + colorOsc * 60);
        var b = Math.floor(160 - colorOsc * 100);
        for (var k = 0; k < platforms.length; k++) {
            var plat = platforms[k];
            // Main body
            ctx.fillStyle = 'rgb(' + r + ',' + g + ',' + b + ')';
            ctx.fillRect(plat.x, plat.y, plat.width, plat.height);
            // Top highlight
            var hr = Math.floor(r + 10);
            var hg = Math.floor(g + 10);
            var hb = Math.floor(b + 10);
            ctx.fillStyle = 'rgb(' + hr + ',' + hg + ',' + hb + ')';
            ctx.fillRect(plat.x, plat.y, plat.width, 3);
        }
        if (orbs && orbs.length > 0) {
            for (var oi = 0; oi < orbs.length; oi++) {
                drawOrb(orbs[oi].x, orbs[oi].y, time, orbs[oi].orbType);
            }
        }
        if (thrownOrbs && thrownOrbs.length > 0) {
            for (var ti = 0; ti < thrownOrbs.length; ti++) drawThrownOrb(thrownOrbs[ti], time);
        }
        if (explosions && explosions.length > 0) {
            for (var ei = 0; ei < explosions.length; ei++) {
                var ex = explosions[ei];
                var alpha = ex.timer / (ex.maxTimer || 0.6);
                var progress = 1 - alpha;
                var isH = ex.isHorizontal;

                // Expanding ring
                ctx.strokeStyle = 'rgba(255,255,200,' + alpha + ')';
                ctx.lineWidth = 3;
                ctx.beginPath();
                ctx.arc(ex.x, ex.y, progress * 80, 0, Math.PI * 2);
                ctx.stroke();

                if (isH) {
                    // Horizontal lightning
                    var grad = ctx.createLinearGradient(0, ex.y - EXPLOSION_WIDTH, 0, ex.y + EXPLOSION_WIDTH);
                    grad.addColorStop(0, 'rgba(255,200,0,0)');
                    grad.addColorStop(0.2, 'rgba(255,255,150,' + (alpha * 0.8) + ')');
                    grad.addColorStop(0.5, 'rgba(255,255,255,' + alpha + ')');
                    grad.addColorStop(0.8, 'rgba(255,255,150,' + (alpha * 0.8) + ')');
                    grad.addColorStop(1, 'rgba(255,200,0,0)');
                    ctx.fillStyle = grad;
                    ctx.fillRect(0, ex.y - 50, worldWidth, 100);
                    ctx.fillStyle = 'rgba(255,255,255,' + (alpha * 0.9) + ')';
                    ctx.fillRect(0, ex.y - 3, worldWidth, 6);
                    // Horizontal lightning bolts
                    ctx.strokeStyle = 'rgba(255,255,255,' + (alpha * 0.6) + ')';
                    ctx.lineWidth = 2;
                    for (var b = 0; b < 3; b++) {
                        ctx.beginPath();
                        var ly = ex.y - 30 + Math.random() * 60;
                        ctx.moveTo(0, ly);
                        for (var xx = 0; xx < worldWidth; xx += 30 + Math.random() * 40) {
                            ly = ex.y - 25 + Math.random() * 50;
                            ctx.lineTo(xx, ly);
                        }
                        ctx.stroke();
                    }
                } else {
                    // Vertical lightning (original)
                    var grad = ctx.createLinearGradient(ex.x - EXPLOSION_WIDTH, 0, ex.x + EXPLOSION_WIDTH, 0);
                    grad.addColorStop(0, 'rgba(255,200,0,0)');
                    grad.addColorStop(0.2, 'rgba(255,255,150,' + (alpha * 0.8) + ')');
                    grad.addColorStop(0.5, 'rgba(255,255,255,' + alpha + ')');
                    grad.addColorStop(0.8, 'rgba(255,255,150,' + (alpha * 0.8) + ')');
                    grad.addColorStop(1, 'rgba(255,200,0,0)');
                    ctx.fillStyle = grad;
                    ctx.fillRect(ex.x - 50, 0, 100, worldHeight);
                    ctx.fillStyle = 'rgba(255,255,255,' + (alpha * 0.9) + ')';
                    ctx.fillRect(ex.x - 3, 0, 6, worldHeight);
                    ctx.strokeStyle = 'rgba(255,255,255,' + (alpha * 0.6) + ')';
                    ctx.lineWidth = 2;
                    for (var b = 0; b < 3; b++) {
                        ctx.beginPath();
                        var lx = ex.x - 30 + Math.random() * 60;
                        ctx.moveTo(lx, 0);
                        for (var yy = 0; yy < worldHeight; yy += 30 + Math.random() * 40) {
                            lx = ex.x - 25 + Math.random() * 50;
                            ctx.lineTo(lx, yy);
                        }
                        ctx.stroke();
                    }
                }
            }
        }
        for (var pid in players) {
            if (players.hasOwnProperty(pid)) drawPlayer(players[pid], pid, time);
        }
        if(bots&&bots.length>0){
            for(var bi=0;bi<bots.length;bi++){
                drawPlayer(bots[bi],bots[bi].id,time);
            }
        }
        ctx.restore();
        var mmW = 150, mmH = 40, mmX = SCREEN_W - mmW - 10, mmY = 10;
        ctx.fillStyle = 'rgba(0,0,0,0.6)';
        ctx.fillRect(mmX, mmY, mmW, mmH);
        ctx.strokeStyle = '#444';
        ctx.strokeRect(mmX, mmY, mmW, mmH);
        for (var pid2 in players) {
            if (players.hasOwnProperty(pid2)) {
                var pp = players[pid2];
                ctx.fillStyle = pp.color || '#fff';
                ctx.fillRect(mmX + (pp.x / worldWidth) * mmW - 2, mmY + (pp.y / worldHeight) * mmH - 2, 4, 4);
            }
        }
        ctx.strokeStyle = '#fff';
        ctx.lineWidth = 1;
        ctx.strokeRect(mmX + (cameraX / worldWidth) * mmW, mmY, (SCREEN_W / worldWidth) * mmW, mmH);
        requestAnimationFrame(draw);
    } catch (err) {
        console.error('Draw error:', err);
        requestAnimationFrame(draw);
    }
}

draw();
console.log('Sounds loaded:', Object.keys(sounds));