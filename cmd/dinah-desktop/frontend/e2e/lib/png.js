// Decode the RGB/RGBA, noninterlaced PNGs emitted by browser screenshots.
const zlib=require('zlib');
module.exports=function png(buffer){
 if(!buffer.subarray(0,8).equals(Buffer.from([137,80,78,71,13,10,26,10])))throw Error('Not a PNG');
 let width,height,channels;const chunks=[];
 for(let offset=8;offset<buffer.length;){const n=buffer.readUInt32BE(offset),type=buffer.toString('ascii',offset+4,offset+8),data=buffer.subarray(offset+8,offset+8+n);offset+=12+n;
  if(type==='IHDR'){width=data.readUInt32BE(0);height=data.readUInt32BE(4);channels=data[9]===6?4:data[9]===2?3:0;if(data[8]!==8||!channels||data[12]!==0)throw Error('Unsupported PNG encoding');}
  if(type==='IDAT')chunks.push(data);if(type==='IEND')break;
 }
 const raw=zlib.inflateSync(Buffer.concat(chunks)),stride=width*channels,pixels=Buffer.alloc(stride*height);let offset=0;
 const paeth=(a,b,c)=>{const p=a+b-c,x=Math.abs(p-a),y=Math.abs(p-b),z=Math.abs(p-c);return x<=y&&x<=z?a:y<=z?b:c;};
 for(let y=0;y<height;y++){const filter=raw[offset++];for(let x=0;x<stride;x++){const a=x>=channels?pixels[y*stride+x-channels]:0,b=y?pixels[(y-1)*stride+x]:0,c=y&&x>=channels?pixels[(y-1)*stride+x-channels]:0;
  const predictor=[0,a,b,Math.floor((a+b)/2),paeth(a,b,c)][filter];if(predictor===undefined)throw Error('Unsupported PNG filter');pixels[y*stride+x]=(raw[offset++]+predictor)&255;
 }}
 return {width,height,pixel:(x,y)=>{const i=(y*width+x)*channels;return [pixels[i],pixels[i+1],pixels[i+2],channels===4?pixels[i+3]:255];}};
};
